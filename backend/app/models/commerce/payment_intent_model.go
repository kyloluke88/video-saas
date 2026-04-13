package commerce

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type PaymentIntent struct {
	models.BaseModel

	OrderID          uint64          `gorm:"column:order_id;not null;index:idx_payment_intents_order_id" json:"order_id"`
	UserID           *uint64         `gorm:"column:user_id;index:idx_payment_intents_user_id" json:"user_id,omitempty"`
	PaymentNo        string          `gorm:"column:payment_no;size:100;not null;uniqueIndex:uk_payment_intents_payment_no" json:"payment_no"`
	Provider         string          `gorm:"column:provider;size:50;not null;index:idx_payment_intents_provider" json:"provider"`
	PaymentMethod    *string         `gorm:"column:payment_method;size:50" json:"payment_method,omitempty"`
	Currency         string          `gorm:"column:currency;size:3;not null" json:"currency"`
	Amount           float64         `gorm:"column:amount;type:numeric(12,2);not null" json:"amount"`
	Status           string          `gorm:"column:status;size:50;not null;default:created;index:idx_payment_intents_status" json:"status"`
	ProviderIntentID *string         `gorm:"column:provider_intent_id;size:255;index:idx_payment_intents_provider_intent_id" json:"provider_intent_id,omitempty"`
	ClientSecret     *string         `gorm:"column:client_secret;type:text" json:"client_secret,omitempty"`
	ExpiresAt        *time.Time      `gorm:"column:expires_at" json:"expires_at,omitempty"`
	SucceededAt      *time.Time      `gorm:"column:succeeded_at" json:"succeeded_at,omitempty"`
	FailedAt         *time.Time      `gorm:"column:failed_at" json:"failed_at,omitempty"`
	CancelledAt      *time.Time      `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	RawResponse      json.RawMessage `gorm:"column:raw_response;type:jsonb;not null;default:'{}'" json:"raw_response,omitempty"`

	models.CommonTimestampsField
}

func (PaymentIntent) TableName() string {
	return "payment_intents"
}
