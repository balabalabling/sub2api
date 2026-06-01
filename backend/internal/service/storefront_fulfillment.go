package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *PaymentService) ExecuteStoreFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed)).SetStatus(OrderStatusRecharging).Save(ctx)
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil
	}
	if err := s.doStoreFulfillment(ctx, o); err != nil {
		s.markStoreOrderDeliveryFailed(ctx, oid, err)
		s.markFailed(ctx, oid, err)
		return err
	}
	return nil
}

func (s *PaymentService) doStoreFulfillment(ctx context.Context, paymentOrder *dbent.PaymentOrder) error {
	storeOrder, err := s.getStoreOrderByPaymentOrderID(ctx, paymentOrder.ID)
	if err != nil {
		return err
	}
	if storeOrder.DeliveryStatus == StoreDeliveryStatusDelivered {
		return s.markCompleted(ctx, paymentOrder, "STORE_DELIVERY_SUCCESS")
	}
	if storeOrder.DeliveryStatus == StoreDeliveryStatusManualRequired {
		return s.markCompleted(ctx, paymentOrder, "STORE_MANUAL_REQUIRED")
	}
	if err := s.setStoreOrderDeliveryStatus(ctx, storeOrder.ID, StoreDeliveryStatusDelivering, ""); err != nil {
		return err
	}

	switch storeOrder.ProductType {
	case StoreProductTypeAPIKey:
		if err := s.deliverStoreAPIKey(ctx, storeOrder); err != nil {
			return err
		}
		return s.markCompleted(ctx, paymentOrder, "STORE_DELIVERY_SUCCESS")
	default:
		if err := s.setStoreOrderDeliveryStatus(ctx, storeOrder.ID, StoreDeliveryStatusManualRequired, ""); err != nil {
			return err
		}
		s.dispatchStoreManualNotification(storeOrder)
		return s.markCompleted(ctx, paymentOrder, "STORE_MANUAL_REQUIRED")
	}
}

func (s *PaymentService) deliverStoreAPIKey(ctx context.Context, order *StoreOrder) error {
	if order.APIKeyID != nil && *order.APIKeyID > 0 {
		return s.markStoreOrderDelivered(ctx, order.ID, *order.APIKeyID, map[string]any{"already_delivered": true})
	}
	userID := int64(0)
	if order.UserID != nil {
		userID = *order.UserID
	}
	if userID <= 0 {
		var err error
		userID, err = s.findOrCreateStoreUser(ctx, order.Email)
		if err != nil {
			return err
		}
	}
	cfg := order.ProductSnapshot
	deliveryConfig, _ := cfg["delivery_config"].(map[string]any)
	groupID := int64FromMap(deliveryConfig, "group_id")
	quota := floatFromMap(deliveryConfig, "quota")
	expiresInDays := intFromMap(deliveryConfig, "expires_in_days")
	rate5h := floatFromMap(deliveryConfig, "rate_limit_5h")
	rate1d := floatFromMap(deliveryConfig, "rate_limit_1d")
	rate7d := floatFromMap(deliveryConfig, "rate_limit_7d")
	if expiresInDays <= 0 {
		expiresInDays = 30
	}
	if groupID > 0 && s.groupRepo != nil {
		if group, err := s.groupRepo.GetByID(ctx, groupID); err != nil || group == nil || group.Status != StatusActive {
			return fmt.Errorf("store product group %d is not active", groupID)
		}
	}
	apiKey, err := generateStoreAPIKey()
	if err != nil {
		return err
	}
	expiresAt := time.Now().AddDate(0, 0, expiresInDays)
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	var apiKeyID int64
	err = db.QueryRowContext(ctx, `
INSERT INTO api_keys (user_id, key, name, group_id, status, quota, quota_used, expires_at, rate_limit_5h, rate_limit_1d, rate_limit_7d)
VALUES ($1,$2,$3,$4,'active',$5,0,$6,$7,$8,$9)
RETURNING id`,
		userID, apiKey, "store-"+order.OrderNo, nullableInt64(groupID), quota, expiresAt, rate5h, rate1d, rate7d).Scan(&apiKeyID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"api_key":         apiKey,
		"api_key_masked":  maskStoreAPIKey(apiKey),
		"expires_at":      expiresAt.Format(time.RFC3339),
		"expires_in_days": expiresInDays,
	}
	if err := s.markStoreOrderDelivered(ctx, order.ID, apiKeyID, payload); err != nil {
		return err
	}
	s.dispatchStoreAPIKeyEmail(order, apiKey, expiresAt)
	return nil
}

func (s *PaymentService) getStoreOrderByPaymentOrderID(ctx context.Context, paymentOrderID int64) (*StoreOrder, error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
SELECT id, order_no, email, product_id, product_type, product_snapshot, amount::double precision, currency, payment_order_id, user_id, api_key_id, delivery_status, delivery_payload, COALESCE(delivery_error,''), email_sent_at, delivered_at, created_at, updated_at
FROM store_orders
WHERE payment_order_id=$1`, paymentOrderID)
	order, err := scanStoreOrder(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("store order for payment order %d not found", paymentOrderID)
		}
		return nil, err
	}
	return order, nil
}

func (s *PaymentService) setStoreOrderDeliveryStatus(ctx context.Context, id int64, status string, errText string) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE store_orders SET delivery_status=$2, delivery_error=NULLIF($3,''), updated_at=NOW() WHERE id=$1`, id, status, errText)
	return err
}

func (s *PaymentService) markStoreOrderDelivered(ctx context.Context, id int64, apiKeyID int64, payload map[string]any) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(payload)
	_, err = db.ExecContext(ctx, `
UPDATE store_orders
SET delivery_status=$2, api_key_id=$3, delivery_payload=delivery_payload || $4::jsonb, delivery_error=NULL,
    delivered_at=COALESCE(delivered_at, NOW()), updated_at=NOW()
WHERE id=$1`, id, StoreDeliveryStatusDelivered, apiKeyID, string(raw))
	return err
}

func (s *PaymentService) markStoreOrderDeliveryFailed(ctx context.Context, paymentOrderID int64, cause error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return
	}
	_, _ = db.ExecContext(ctx, `UPDATE store_orders SET delivery_status=$2, delivery_error=$3, updated_at=NOW() WHERE payment_order_id=$1`, paymentOrderID, StoreDeliveryStatusFailed, cause.Error())
}

func (s *PaymentService) dispatchStoreAPIKeyEmail(order *StoreOrder, apiKey string, expiresAt time.Time) {
	if s.notificationEmailService == nil || order == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()
		productName, _ := order.ProductSnapshot["name"].(string)
		err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventStoreAPIKeyDelivered,
			RecipientEmail: order.Email,
			RecipientName:  order.Email,
			UserID:         derefInt64(order.UserID),
			SourceType:     "store_order",
			SourceID:       strconv.FormatInt(order.ID, 10),
			Variables: map[string]string{
				"order_no":     order.OrderNo,
				"product_name": productName,
				"api_key":      apiKey,
				"expires_at":   expiresAt.Format("2006-01-02 15:04"),
			},
		})
		if err != nil {
			slog.Warn("store api key email failed", "store_order_id", order.ID, "err", err.Error())
			return
		}
		if db, dbErr := s.storeSQLDB(); dbErr == nil {
			_, _ = db.ExecContext(ctx, `UPDATE store_orders SET email_sent_at=NOW(), updated_at=NOW() WHERE id=$1`, order.ID)
		}
	}()
}

func (s *PaymentService) dispatchStoreManualNotification(order *StoreOrder) {
	if s.notificationEmailService == nil || order == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()
		productName, _ := order.ProductSnapshot["name"].(string)
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventStoreManualRequired,
			RecipientEmail: order.Email,
			RecipientName:  order.Email,
			UserID:         derefInt64(order.UserID),
			SourceType:     "store_order",
			SourceID:       strconv.FormatInt(order.ID, 10),
			Variables: map[string]string{
				"order_no":     order.OrderNo,
				"product_name": productName,
			},
		}); err != nil {
			slog.Warn("store manual notification email failed", "store_order_id", order.ID, "err", err.Error())
		}
	}()
}

func generateStoreAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(buf), nil
}

func nullableInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func int64FromMap(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}

func intFromMap(m map[string]any, key string) int {
	return int(int64FromMap(m, key))
}

func floatFromMap(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	default:
		return 0
	}
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
