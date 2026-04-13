package commerce

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type Order struct {
	models.BaseModel

	OrderNo                 string          `gorm:"column:order_no;size:100;not null;uniqueIndex:uk_orders_order_no" json:"order_no"`
	UserID                  *uint64         `gorm:"column:user_id;index:idx_orders_user_id" json:"user_id,omitempty"`
	Status                  string          `gorm:"column:status;size:50;not null;default:pending_payment;index:idx_orders_status" json:"status"`
	Currency                string          `gorm:"column:currency;size:3;not null;default:USD" json:"currency"`
	SubtotalAmount          float64         `gorm:"column:subtotal_amount;type:numeric(12,2);not null;default:0" json:"subtotal_amount"`
	DiscountAmount          float64         `gorm:"column:discount_amount;type:numeric(12,2);not null;default:0" json:"discount_amount"`
	ShippingAmount          float64         `gorm:"column:shipping_amount;type:numeric(12,2);not null;default:0" json:"shipping_amount"`
	TaxAmount               float64         `gorm:"column:tax_amount;type:numeric(12,2);not null;default:0" json:"tax_amount"`
	TotalAmount             float64         `gorm:"column:total_amount;type:numeric(12,2);not null;default:0" json:"total_amount"`
	ContactEmail            *string         `gorm:"column:contact_email;size:255;index:idx_orders_contact_email" json:"contact_email,omitempty"`
	ShippingAddressSnapshot json.RawMessage `gorm:"column:shipping_address_snapshot;type:jsonb" json:"shipping_address_snapshot,omitempty"`
	BillingAddressSnapshot  json.RawMessage `gorm:"column:billing_address_snapshot;type:jsonb" json:"billing_address_snapshot,omitempty"`
	Remark                  *string         `gorm:"column:remark;type:text" json:"remark,omitempty"`
	PaidAt                  *time.Time      `gorm:"column:paid_at;index:idx_orders_paid_at" json:"paid_at,omitempty"`
	CancelledAt             *time.Time      `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	ClosedAt                *time.Time      `gorm:"column:closed_at" json:"closed_at,omitempty"`

	models.CommonTimestampsField
}

func (Order) TableName() string {
	return "orders"
}
