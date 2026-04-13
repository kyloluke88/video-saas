package commerce

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type PaymentTransaction struct {
	models.BaseModel

	PaymentIntentID       uint64          `gorm:"column:payment_intent_id;not null;index:idx_payment_transactions_payment_intent_id" json:"payment_intent_id"`
	OrderID               uint64          `gorm:"column:order_id;not null;index:idx_payment_transactions_order_id" json:"order_id"`
	Provider              string          `gorm:"column:provider;size:50;not null;index:idx_payment_transactions_provider" json:"provider"`
	ProviderTransactionID *string         `gorm:"column:provider_transaction_id;size:255;index:idx_payment_transactions_provider_transaction_id" json:"provider_transaction_id,omitempty"`
	TransactionType       string          `gorm:"column:transaction_type;size:50;not null;index:idx_payment_transactions_transaction_type" json:"transaction_type"`
	Status                string          `gorm:"column:status;size:50;not null;index:idx_payment_transactions_status" json:"status"`
	Amount                float64         `gorm:"column:amount;type:numeric(12,2);not null" json:"amount"`
	Currency              string          `gorm:"column:currency;size:3;not null" json:"currency"`
	ProviderPayload       json.RawMessage `gorm:"column:provider_payload;type:jsonb;not null;default:'{}'" json:"provider_payload,omitempty"`
	HappenedAt            *time.Time      `gorm:"column:happened_at;index:idx_payment_transactions_happened_at" json:"happened_at,omitempty"`
	CreatedAt             time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at,omitempty"`
}

func (PaymentTransaction) TableName() string {
	return "payment_transactions"
}
