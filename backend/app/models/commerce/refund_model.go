package commerce

import "api/app/models"

type Refund struct {
	models.BaseModel

	OrderID              uint64  `gorm:"column:order_id;not null;index:idx_refunds_order_id" json:"order_id"`
	PaymentTransactionID *uint64 `gorm:"column:payment_transaction_id;index:idx_refunds_payment_transaction_id" json:"payment_transaction_id,omitempty"`
	RefundNo             string  `gorm:"column:refund_no;size:100;not null;uniqueIndex:uk_refunds_refund_no" json:"refund_no"`
	Amount               float64 `gorm:"column:amount;type:numeric(12,2);not null" json:"amount"`
	Currency             string  `gorm:"column:currency;size:3;not null" json:"currency"`
	Reason               *string `gorm:"column:reason;type:text" json:"reason,omitempty"`
	Status               string  `gorm:"column:status;size:50;not null;default:pending;index:idx_refunds_status" json:"status"`
	ProviderRefundID     *string `gorm:"column:provider_refund_id;size:255" json:"provider_refund_id,omitempty"`

	models.CommonTimestampsField
}

func (Refund) TableName() string {
	return "refunds"
}
