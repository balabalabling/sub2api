package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	entsql "entgo.io/ent/dialect/sql"
	"golang.org/x/crypto/bcrypt"
)

const (
	StoreProductTypeAPIKey  = "api_key"
	StoreProductTypeAccount = "account"
	StoreProductTypeSMS     = "sms"
	StoreProductTypeManual  = "manual"

	StoreProductStatusDraft    = "draft"
	StoreProductStatusActive   = "active"
	StoreProductStatusInactive = "inactive"

	StoreVisibilityPublic = "public"
	StoreVisibilityHidden = "hidden"

	StoreStockModeUnlimited = "unlimited"
	StoreStockModeTracked   = "tracked"

	StoreDeliveryModeAuto   = "auto"
	StoreDeliveryModeManual = "manual"

	StoreDeliveryStatusPending        = "pending"
	StoreDeliveryStatusPaid           = "paid"
	StoreDeliveryStatusDelivering     = "delivering"
	StoreDeliveryStatusDelivered      = "delivered"
	StoreDeliveryStatusFailed         = "failed"
	StoreDeliveryStatusManualRequired = "manual_required"

	storeEmailCodePurposeQuery = "query"
	storeEmailCodeTTL          = 10 * time.Minute
)

type StoreProduct struct {
	ID             int64          `json:"id"`
	ProductType    string         `json:"product_type"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Price          float64        `json:"price"`
	Currency       string         `json:"currency"`
	Status         string         `json:"status"`
	Visibility     string         `json:"visibility"`
	SortOrder      int            `json:"sort_order"`
	StockMode      string         `json:"stock_mode"`
	StockCount     int            `json:"stock_count"`
	DeliveryMode   string         `json:"delivery_mode"`
	DeliveryConfig map[string]any `json:"delivery_config"`
	SaleStartAt    *time.Time     `json:"sale_start_at,omitempty"`
	SaleEndAt      *time.Time     `json:"sale_end_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type StoreProductInput struct {
	ProductType    string         `json:"product_type"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Price          float64        `json:"price"`
	Currency       string         `json:"currency"`
	Status         string         `json:"status"`
	Visibility     string         `json:"visibility"`
	SortOrder      int            `json:"sort_order"`
	StockMode      string         `json:"stock_mode"`
	StockCount     int            `json:"stock_count"`
	DeliveryMode   string         `json:"delivery_mode"`
	DeliveryConfig map[string]any `json:"delivery_config"`
	SaleStartAt    *time.Time     `json:"sale_start_at"`
	SaleEndAt      *time.Time     `json:"sale_end_at"`
}

type StoreOrder struct {
	ID              int64          `json:"id"`
	OrderNo         string         `json:"order_no"`
	Email           string         `json:"email"`
	ProductID       *int64         `json:"product_id,omitempty"`
	ProductType     string         `json:"product_type"`
	ProductSnapshot map[string]any `json:"product_snapshot"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	PaymentOrderID  *int64         `json:"payment_order_id,omitempty"`
	UserID          *int64         `json:"user_id,omitempty"`
	APIKeyID        *int64         `json:"api_key_id,omitempty"`
	DeliveryStatus  string         `json:"delivery_status"`
	DeliveryPayload map[string]any `json:"delivery_payload"`
	DeliveryError   string         `json:"delivery_error,omitempty"`
	EmailSentAt     *time.Time     `json:"email_sent_at,omitempty"`
	DeliveredAt     *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type StorefrontCreateOrderInput struct {
	Email       string
	ProductID   int64
	PaymentType string
	ClientIP    string
	IsMobile    bool
	SrcHost     string
	SrcURL      string
	ReturnURL   string
	Locale      string
}

type StorefrontCreateOrderResult struct {
	StoreOrder  *StoreOrder          `json:"store_order"`
	Payment     *CreateOrderResponse `json:"payment"`
	PaymentMode string               `json:"payment_mode,omitempty"`
}

type StoreUsageItem struct {
	OrderNo        string     `json:"order_no,omitempty"`
	ProductType    string     `json:"product_type,omitempty"`
	ProductName    string     `json:"product_name,omitempty"`
	DeliveryStatus string     `json:"delivery_status,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	APIKeyID       int64      `json:"api_key_id,omitempty"`
	APIKeyMasked   string     `json:"api_key_masked,omitempty"`
	KeyStatus      string     `json:"key_status,omitempty"`
	Quota          float64    `json:"quota"`
	QuotaUsed      float64    `json:"quota_used"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	Balance        float64    `json:"balance"`
	InputTokens    int64      `json:"input_tokens"`
	OutputTokens   int64      `json:"output_tokens"`
	TotalCost      float64    `json:"total_cost"`
}

func (s *PaymentService) storeSQLDB() (*sql.DB, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("nil payment service")
	}
	drv, ok := s.entClient.Driver().(*entsql.Driver)
	if !ok || drv == nil {
		return nil, fmt.Errorf("ent driver is not sql driver")
	}
	return drv.DB(), nil
}

func (s *PaymentService) ListStoreProducts(ctx context.Context, publicOnly bool) ([]StoreProduct, error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	query := `
SELECT id, product_type, name, description, price::double precision, currency, status, visibility,
       sort_order, stock_mode, stock_count, delivery_mode, delivery_config, sale_start_at, sale_end_at, created_at, updated_at
FROM store_products
WHERE deleted_at IS NULL`
	args := []any{}
	if publicOnly {
		query += ` AND status = 'active' AND visibility = 'public' AND (sale_start_at IS NULL OR sale_start_at <= NOW()) AND (sale_end_at IS NULL OR sale_end_at >= NOW())`
	}
	query += ` ORDER BY sort_order ASC, id ASC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoreProduct
	for rows.Next() {
		p, err := scanStoreProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *PaymentService) GetStoreProduct(ctx context.Context, id int64, publicOnly bool) (*StoreProduct, error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	query := `
SELECT id, product_type, name, description, price::double precision, currency, status, visibility,
       sort_order, stock_mode, stock_count, delivery_mode, delivery_config, sale_start_at, sale_end_at, created_at, updated_at
FROM store_products
WHERE id = $1 AND deleted_at IS NULL`
	if publicOnly {
		query += ` AND status = 'active' AND visibility = 'public' AND (sale_start_at IS NULL OR sale_start_at <= NOW()) AND (sale_end_at IS NULL OR sale_end_at >= NOW())`
	}
	row := db.QueryRowContext(ctx, query, id)
	p, err := scanStoreProduct(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, infraerrors.NotFound("PRODUCT_NOT_FOUND", "product not found")
		}
		return nil, err
	}
	return &p, nil
}

func (s *PaymentService) CreateStoreProduct(ctx context.Context, input StoreProductInput) (*StoreProduct, error) {
	if err := validateStoreProductInput(input); err != nil {
		return nil, err
	}
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	cfg, _ := json.Marshal(input.DeliveryConfig)
	row := db.QueryRowContext(ctx, `
INSERT INTO store_products (product_type, name, description, price, currency, status, visibility, sort_order, stock_mode, stock_count, delivery_mode, delivery_config, sale_start_at, sale_end_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13,$14)
RETURNING id, product_type, name, description, price::double precision, currency, status, visibility,
       sort_order, stock_mode, stock_count, delivery_mode, delivery_config, sale_start_at, sale_end_at, created_at, updated_at`,
		input.ProductType, strings.TrimSpace(input.Name), input.Description, input.Price, normalizeCurrency(input.Currency),
		normalizeStoreProductStatus(input.Status), normalizeStoreVisibility(input.Visibility), input.SortOrder,
		normalizeStoreStockMode(input.StockMode), input.StockCount, normalizeStoreDeliveryMode(input.DeliveryMode),
		string(cfg), input.SaleStartAt, input.SaleEndAt,
	)
	p, err := scanStoreProduct(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PaymentService) UpdateStoreProduct(ctx context.Context, id int64, input StoreProductInput) (*StoreProduct, error) {
	if err := validateStoreProductInput(input); err != nil {
		return nil, err
	}
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	cfg, _ := json.Marshal(input.DeliveryConfig)
	row := db.QueryRowContext(ctx, `
UPDATE store_products
SET product_type=$2, name=$3, description=$4, price=$5, currency=$6, status=$7, visibility=$8, sort_order=$9,
    stock_mode=$10, stock_count=$11, delivery_mode=$12, delivery_config=$13::jsonb, sale_start_at=$14, sale_end_at=$15, updated_at=NOW()
WHERE id=$1 AND deleted_at IS NULL
RETURNING id, product_type, name, description, price::double precision, currency, status, visibility,
       sort_order, stock_mode, stock_count, delivery_mode, delivery_config, sale_start_at, sale_end_at, created_at, updated_at`,
		id, input.ProductType, strings.TrimSpace(input.Name), input.Description, input.Price, normalizeCurrency(input.Currency),
		normalizeStoreProductStatus(input.Status), normalizeStoreVisibility(input.Visibility), input.SortOrder,
		normalizeStoreStockMode(input.StockMode), input.StockCount, normalizeStoreDeliveryMode(input.DeliveryMode),
		string(cfg), input.SaleStartAt, input.SaleEndAt,
	)
	p, err := scanStoreProduct(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, infraerrors.NotFound("PRODUCT_NOT_FOUND", "product not found")
		}
		return nil, err
	}
	return &p, nil
}

func (s *PaymentService) DeleteStoreProduct(ctx context.Context, id int64) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, `UPDATE store_products SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return infraerrors.NotFound("PRODUCT_NOT_FOUND", "product not found")
	}
	return nil
}

func (s *PaymentService) CreateStorefrontOrder(ctx context.Context, input StorefrontCreateOrderInput) (*StorefrontCreateOrderResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !isLikelyEmail(email) {
		return nil, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}
	product, err := s.GetStoreProduct(ctx, input.ProductID, true)
	if err != nil {
		return nil, err
	}
	if product.StockMode == StoreStockModeTracked && product.StockCount <= 0 {
		return nil, infraerrors.BadRequest("OUT_OF_STOCK", "product is out of stock")
	}
	stockReserved := false
	if product.StockMode == StoreStockModeTracked {
		if err := s.reserveStoreProductStock(ctx, product.ID); err != nil {
			return nil, err
		}
		stockReserved = true
	}
	releaseStock := func() {
		if stockReserved {
			_ = s.releaseStoreProductStock(context.Background(), product.ID)
		}
	}
	userID, err := s.findOrCreateStoreUser(ctx, email)
	if err != nil {
		releaseStock()
		return nil, err
	}
	storeOrder, err := s.createStoreOrder(ctx, email, product, userID)
	if err != nil {
		releaseStock()
		return nil, err
	}
	payType := strings.TrimSpace(input.PaymentType)
	if payType == "" {
		payType = payment.TypeAlipay
	}
	paymentResp, err := s.CreateOrder(ctx, CreateOrderRequest{
		UserID:      userID,
		Amount:      product.Price,
		PaymentType: payType,
		ClientIP:    input.ClientIP,
		IsMobile:    input.IsMobile,
		SrcHost:     input.SrcHost,
		SrcURL:      input.SrcURL,
		ReturnURL:   input.ReturnURL,
		OrderType:   payment.OrderTypeStore,
		Locale:      input.Locale,
	})
	if err != nil {
		_ = s.markStoreOrderFailed(ctx, storeOrder.ID, err)
		releaseStock()
		return nil, err
	}
	storeOrder, err = s.attachPaymentOrder(ctx, storeOrder.ID, paymentResp.OrderID)
	if err != nil {
		releaseStock()
		return nil, err
	}
	return &StorefrontCreateOrderResult{StoreOrder: storeOrder, Payment: paymentResp}, nil
}

func (s *PaymentService) SendStoreQueryCode(ctx context.Context, email string, locale string) error {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if !isLikelyEmail(normalized) {
		return infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}
	code := randomDigits(6)
	sum := sha256.Sum256([]byte(normalized + ":" + code))
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO store_email_codes (email, code_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)`, normalized, hex.EncodeToString(sum[:]), storeEmailCodePurposeQuery, time.Now().Add(storeEmailCodeTTL)); err != nil {
		return err
	}
	if s.notificationEmailService == nil {
		slog.Warn("store query code generated but notification email service is unavailable", "email_hash", storeEmailHash(normalized))
		return nil
	}
	return s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventStoreQueryCode,
		Locale:         locale,
		RecipientEmail: normalized,
		RecipientName:  normalized,
		SourceType:     "store_query",
		SourceID:       normalized,
		ReminderKey:    strconv.FormatInt(time.Now().UnixNano(), 10),
		Variables: map[string]string{
			"verification_code":  code,
			"expires_in_minutes": strconv.Itoa(int(storeEmailCodeTTL / time.Minute)),
		},
	})
}

func (s *PaymentService) VerifyStoreQueryCode(ctx context.Context, email string, code string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	if !isLikelyEmail(normalized) || len(code) != 6 {
		return "", infraerrors.BadRequest("INVALID_CODE", "invalid code")
	}
	sum := sha256.Sum256([]byte(normalized + ":" + code))
	db, err := s.storeSQLDB()
	if err != nil {
		return "", err
	}
	var id int64
	err = db.QueryRowContext(ctx, `
SELECT id FROM store_email_codes
WHERE lower(email)=lower($1) AND purpose=$2 AND code_hash=$3 AND consumed_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1`, normalized, storeEmailCodePurposeQuery, hex.EncodeToString(sum[:])).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", infraerrors.BadRequest("INVALID_CODE", "invalid or expired code")
		}
		return "", err
	}
	_, _ = db.ExecContext(ctx, `UPDATE store_email_codes SET consumed_at=NOW() WHERE id=$1`, id)
	token := makeStoreQueryToken(normalized)
	return token, nil
}

func (s *PaymentService) ListStoreUsageByEmail(ctx context.Context, email, token string) ([]StoreUsageItem, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if !verifyStoreQueryToken(normalized, token) {
		return nil, infraerrors.Forbidden("INVALID_QUERY_TOKEN", "invalid query token")
	}
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, storeUsageByEmailSQL, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoreUsageRows(rows)
}

func (s *PaymentService) ListStoreUsageByKey(ctx context.Context, key string) ([]StoreUsageItem, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, infraerrors.BadRequest("INVALID_KEY", "invalid api key")
	}
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, storeUsageByKeySQL, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoreUsageRows(rows)
}

func (s *PaymentService) createStoreOrder(ctx context.Context, email string, p *StoreProduct, userID int64) (*StoreOrder, error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	snapshot, _ := json.Marshal(p)
	orderNo := "STORE-" + time.Now().Format("20060102") + "-" + randomString(8)
	row := db.QueryRowContext(ctx, `
INSERT INTO store_orders (order_no, email, product_id, product_type, product_snapshot, amount, currency, user_id)
VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8)
RETURNING id, order_no, email, product_id, product_type, product_snapshot, amount::double precision, currency, payment_order_id, user_id, api_key_id, delivery_status, delivery_payload, COALESCE(delivery_error,''), email_sent_at, delivered_at, created_at, updated_at`,
		orderNo, email, p.ID, p.ProductType, string(snapshot), p.Price, p.Currency, userID,
	)
	return scanStoreOrder(row)
}

func (s *PaymentService) attachPaymentOrder(ctx context.Context, storeOrderID, paymentOrderID int64) (*StoreOrder, error) {
	db, err := s.storeSQLDB()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
UPDATE store_orders
SET payment_order_id=$2, updated_at=NOW()
WHERE id=$1
RETURNING id, order_no, email, product_id, product_type, product_snapshot, amount::double precision, currency, payment_order_id, user_id, api_key_id, delivery_status, delivery_payload, COALESCE(delivery_error,''), email_sent_at, delivered_at, created_at, updated_at`,
		storeOrderID, paymentOrderID)
	return scanStoreOrder(row)
}

func (s *PaymentService) findOrCreateStoreUser(ctx context.Context, email string) (int64, error) {
	if s.userRepo != nil {
		if user, err := s.userRepo.GetByEmail(ctx, email); err == nil && user != nil {
			return user.ID, nil
		}
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(randomString(32)), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	db, err := s.storeSQLDB()
	if err != nil {
		return 0, err
	}
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL ORDER BY id ASC LIMIT 1`, email).Scan(&id); err == nil {
		return id, nil
	} else if err != sql.ErrNoRows {
		return 0, err
	}
	err = db.QueryRowContext(ctx, `
	INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, signup_source)
	VALUES ($1,$2,'user',0,5,'active','','created by storefront; login is not provided to customer','email')
	RETURNING id`, email, string(passwordHash)).Scan(&id)
	if err != nil {
		var existing int64
		if scanErr := db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL ORDER BY id ASC LIMIT 1`, email).Scan(&existing); scanErr == nil {
			return existing, nil
		}
		return 0, err
	}
	return id, nil
}

func (s *PaymentService) reserveStoreProductStock(ctx context.Context, productID int64) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, `
	UPDATE store_products
	SET stock_count = stock_count - 1, updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL AND stock_mode = $2 AND stock_count > 0`, productID, StoreStockModeTracked)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return infraerrors.BadRequest("OUT_OF_STOCK", "product is out of stock")
	}
	return nil
}

func (s *PaymentService) releaseStoreProductStock(ctx context.Context, productID int64) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
	UPDATE store_products
	SET stock_count = stock_count + 1, updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL AND stock_mode = $2`, productID, StoreStockModeTracked)
	return err
}

func (s *PaymentService) markStoreOrderFailed(ctx context.Context, storeOrderID int64, cause error) error {
	db, err := s.storeSQLDB()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE store_orders SET delivery_status=$2, delivery_error=$3, updated_at=NOW() WHERE id=$1`, storeOrderID, StoreDeliveryStatusFailed, cause.Error())
	return err
}

func scanStoreProduct(scanner interface{ Scan(dest ...any) error }) (StoreProduct, error) {
	var p StoreProduct
	var cfg []byte
	if err := scanner.Scan(&p.ID, &p.ProductType, &p.Name, &p.Description, &p.Price, &p.Currency, &p.Status, &p.Visibility, &p.SortOrder, &p.StockMode, &p.StockCount, &p.DeliveryMode, &cfg, &p.SaleStartAt, &p.SaleEndAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return p, err
	}
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &p.DeliveryConfig)
	}
	if p.DeliveryConfig == nil {
		p.DeliveryConfig = map[string]any{}
	}
	return p, nil
}

func scanStoreOrder(scanner interface{ Scan(dest ...any) error }) (*StoreOrder, error) {
	var o StoreOrder
	var productID, paymentOrderID, userID, apiKeyID sql.NullInt64
	var snap, payload []byte
	if err := scanner.Scan(&o.ID, &o.OrderNo, &o.Email, &productID, &o.ProductType, &snap, &o.Amount, &o.Currency, &paymentOrderID, &userID, &apiKeyID, &o.DeliveryStatus, &payload, &o.DeliveryError, &o.EmailSentAt, &o.DeliveredAt, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	if productID.Valid {
		o.ProductID = &productID.Int64
	}
	if paymentOrderID.Valid {
		o.PaymentOrderID = &paymentOrderID.Int64
	}
	if userID.Valid {
		o.UserID = &userID.Int64
	}
	if apiKeyID.Valid {
		o.APIKeyID = &apiKeyID.Int64
	}
	_ = json.Unmarshal(snap, &o.ProductSnapshot)
	_ = json.Unmarshal(payload, &o.DeliveryPayload)
	if o.ProductSnapshot == nil {
		o.ProductSnapshot = map[string]any{}
	}
	if o.DeliveryPayload == nil {
		o.DeliveryPayload = map[string]any{}
	}
	return &o, nil
}

func validateStoreProductInput(input StoreProductInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return infraerrors.BadRequest("INVALID_PRODUCT", "name is required")
	}
	if math.IsNaN(input.Price) || math.IsInf(input.Price, 0) || input.Price <= 0 {
		return infraerrors.BadRequest("INVALID_PRODUCT", "price must be positive")
	}
	pt := normalizeStoreProductType(input.ProductType)
	if pt == "" {
		return infraerrors.BadRequest("INVALID_PRODUCT", "invalid product type")
	}
	return nil
}

func normalizeStoreProductType(v string) string {
	switch strings.TrimSpace(v) {
	case "", StoreProductTypeAPIKey:
		return StoreProductTypeAPIKey
	case StoreProductTypeAccount, StoreProductTypeSMS, StoreProductTypeManual:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func normalizeStoreProductStatus(v string) string {
	switch strings.TrimSpace(v) {
	case StoreProductStatusActive, StoreProductStatusInactive:
		return strings.TrimSpace(v)
	default:
		return StoreProductStatusDraft
	}
}

func normalizeStoreVisibility(v string) string {
	if strings.TrimSpace(v) == StoreVisibilityHidden {
		return StoreVisibilityHidden
	}
	return StoreVisibilityPublic
}

func normalizeStoreStockMode(v string) string {
	if strings.TrimSpace(v) == StoreStockModeTracked {
		return StoreStockModeTracked
	}
	return StoreStockModeUnlimited
}

func normalizeStoreDeliveryMode(v string) string {
	if strings.TrimSpace(v) == StoreDeliveryModeManual {
		return StoreDeliveryModeManual
	}
	return StoreDeliveryModeAuto
}

func normalizeCurrency(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return "CNY"
	}
	return v
}

func isLikelyEmail(email string) bool {
	return len(email) >= 3 && len(email) <= 320 && strings.Contains(email, "@") && strings.Contains(email, ".")
}

func randomString(n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789abcdefghijkmnopqrstuvwxyz"
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

func randomDigits(n int) string {
	const digits = "0123456789"
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	for i := range buf {
		buf[i] = digits[int(buf[i])%len(digits)]
	}
	return string(buf)
}

func storeEmailHash(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:8])
}

func makeStoreQueryToken(email string) string {
	expires := time.Now().Add(30 * time.Minute).Unix()
	payload := strings.ToLower(strings.TrimSpace(email)) + ":" + strconv.FormatInt(expires, 10)
	sum := sha256.Sum256([]byte(payload + ":storefront-query-v1"))
	return strconv.FormatInt(expires, 10) + "." + hex.EncodeToString(sum[:])
}

func verifyStoreQueryToken(email, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return false
	}
	payload := strings.ToLower(strings.TrimSpace(email)) + ":" + parts[0]
	sum := sha256.Sum256([]byte(payload + ":storefront-query-v1"))
	return parts[1] == hex.EncodeToString(sum[:])
}

func maskStoreAPIKey(key string) string {
	if len(key) <= 14 {
		return "********"
	}
	return key[:8] + "..." + key[len(key)-6:]
}

const storeUsageByEmailSQL = `
SELECT COALESCE(o.order_no,''), COALESCE(o.product_type,''), COALESCE(o.product_snapshot->>'name',''),
       COALESCE(o.delivery_status,''), o.created_at, po.paid_at, o.delivered_at,
       COALESCE(k.id,0), COALESCE(k.key,''), COALESCE(k.status,''), COALESCE(k.quota,0)::double precision,
       COALESCE(k.quota_used,0)::double precision, k.expires_at, k.last_used_at,
       COALESCE(u.balance,0)::double precision,
       COALESCE(SUM(l.input_tokens),0)::bigint, COALESCE(SUM(l.output_tokens),0)::bigint, COALESCE(SUM(l.total_cost),0)::double precision
FROM store_orders o
LEFT JOIN payment_orders po ON po.id=o.payment_order_id
LEFT JOIN users u ON u.id=o.user_id
LEFT JOIN api_keys k ON k.id=o.api_key_id
LEFT JOIN usage_logs l ON l.api_key_id=k.id
WHERE lower(o.email)=lower($1)
GROUP BY o.id, po.id, u.id, k.id
ORDER BY o.created_at DESC`

const storeUsageByKeySQL = `
SELECT COALESCE(o.order_no,''), COALESCE(o.product_type,''), COALESCE(o.product_snapshot->>'name',''),
       COALESCE(o.delivery_status,''), o.created_at, po.paid_at, o.delivered_at,
       COALESCE(k.id,0), COALESCE(k.key,''), COALESCE(k.status,''), COALESCE(k.quota,0)::double precision,
       COALESCE(k.quota_used,0)::double precision, k.expires_at, k.last_used_at,
       COALESCE(u.balance,0)::double precision,
       COALESCE(SUM(l.input_tokens),0)::bigint, COALESCE(SUM(l.output_tokens),0)::bigint, COALESCE(SUM(l.total_cost),0)::double precision
FROM api_keys k
LEFT JOIN users u ON u.id=k.user_id
LEFT JOIN store_orders o ON o.api_key_id=k.id
LEFT JOIN payment_orders po ON po.id=o.payment_order_id
LEFT JOIN usage_logs l ON l.api_key_id=k.id
WHERE k.key=$1
GROUP BY o.id, po.id, u.id, k.id
ORDER BY o.created_at DESC NULLS LAST
LIMIT 1`

func scanStoreUsageRows(rows *sql.Rows) ([]StoreUsageItem, error) {
	var out []StoreUsageItem
	for rows.Next() {
		var item StoreUsageItem
		var key string
		if err := rows.Scan(&item.OrderNo, &item.ProductType, &item.ProductName, &item.DeliveryStatus, &item.CreatedAt, &item.PaidAt, &item.DeliveredAt, &item.APIKeyID, &key, &item.KeyStatus, &item.Quota, &item.QuotaUsed, &item.ExpiresAt, &item.LastUsedAt, &item.Balance, &item.InputTokens, &item.OutputTokens, &item.TotalCost); err != nil {
			return nil, err
		}
		item.APIKeyMasked = maskStoreAPIKey(key)
		out = append(out, item)
	}
	return out, rows.Err()
}
