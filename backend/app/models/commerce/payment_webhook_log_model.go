package commerce

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type PaymentWebhookLog struct {
	models.BaseModel

	Provider     string          `gorm:"column:provider;size:50;not null;index:idx_payment_webhook_logs_provider" json:"provider"`
	EventID      *string         `gorm:"column:event_id;size:255;index:idx_payment_webhook_logs_event_id" json:"event_id,omitempty"`
	EventType    *string         `gorm:"column:event_type;size:100" json:"event_type,omitempty"`
	Payload      json.RawMessage `gorm:"column:payload;type:jsonb;not null" json:"payload,omitempty"`
	Processed    bool            `gorm:"column:processed;not null;default:false;index:idx_payment_webhook_logs_processed" json:"processed"`
	ProcessedAt  *time.Time      `gorm:"column:processed_at" json:"processed_at,omitempty"`
	ErrorMessage *string         `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time       `gorm:"column:created_at;not null;default:now();index:idx_payment_webhook_logs_created_at" json:"created_at,omitempty"`
}

func (PaymentWebhookLog) TableName() string {
	return "payment_webhook_logs"
}
