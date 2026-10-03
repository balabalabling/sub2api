package migrate

import "testing"

func TestOfficialPaymentOrderGeneratedIndexes(t *testing.T) {
	expected := map[string][]string{
		"paymentorder_out_trade_no":         {"out_trade_no"},
		"paymentorder_user_id":              {"user_id"},
		"paymentorder_status":               {"status"},
		"paymentorder_expires_at":           {"expires_at"},
		"paymentorder_created_at":           {"created_at"},
		"paymentorder_paid_at":              {"paid_at"},
		"paymentorder_payment_type_paid_at": {"payment_type", "paid_at"},
		"paymentorder_order_type":           {"order_type"},
	}
	for _, index := range PaymentOrdersTable.Indexes {
		want, ok := expected[index.Name]
		if !ok {
			continue
		}
		if len(index.Columns) != len(want) {
			t.Fatalf("index %s: got %d columns, want %d", index.Name, len(index.Columns), len(want))
		}
		for i, column := range index.Columns {
			if column.Name != want[i] {
				t.Errorf("index %s column %d: got %s, want %s", index.Name, i, column.Name, want[i])
			}
		}
		delete(expected, index.Name)
	}
	for name := range expected {
		t.Errorf("missing index %s", name)
	}
	for _, fk := range PaymentOrdersTable.ForeignKeys {
		if len(fk.Columns) != 1 || fk.Columns[0].Name != "user_id" {
			t.Errorf("payment order foreign key must refer to user_id, got %v", fk.Columns)
		}
	}
}

func TestOfficialPaymentModelsRetireCustomKeyDeliveryFields(t *testing.T) {
	for _, column := range PaymentOrdersColumns {
		if column.Name == "api_key_id" {
			t.Error("official payment order schema should not keep the retired custom key delivery field")
		}
	}
	for _, column := range SubscriptionPlansColumns {
		if column.Name == "key_quota_usd" {
			t.Error("official subscription plan schema should not keep the retired custom key quota field")
		}
	}
}
