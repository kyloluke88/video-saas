package commerce

import (
	"encoding/json"
	"time"

	"api/app/models"
)

type OrderItem struct {
	models.BaseModel

	OrderID           uint64          `gorm:"column:order_id;not null;index:idx_order_items_order_id" json:"order_id"`
	ProductID         uint64          `gorm:"column:product_id;not null;index:idx_order_items_product_id" json:"product_id"`
	SKUID             uint64          `gorm:"column:sku_id;not null;index:idx_order_items_sku_id" json:"sku_id"`
	ProductName       string          `gorm:"column:product_name;size:255;not null" json:"product_name"`
	SKUName           string          `gorm:"column:sku_name;size:255;not null" json:"sku_name"`
	SKUCode           string          `gorm:"column:sku_code;size:100;not null;index:idx_order_items_sku_code" json:"sku_code"`
	Quantity          int             `gorm:"column:quantity;not null" json:"quantity"`
	UnitPrice         float64         `gorm:"column:unit_price;type:numeric(12,2);not null" json:"unit_price"`
	OriginalUnitPrice *float64        `gorm:"column:original_unit_price;type:numeric(12,2)" json:"original_unit_price,omitempty"`
	LineTotal         float64         `gorm:"column:line_total;type:numeric(12,2);not null" json:"line_total"`
	ProductType       string          `gorm:"column:product_type;size:50;not null" json:"product_type"`
	IsVirtual         bool            `gorm:"column:is_virtual;not null;default:true" json:"is_virtual"`
	ItemSnapshot      json.RawMessage `gorm:"column:item_snapshot;type:jsonb;not null;default:'{}'" json:"item_snapshot,omitempty"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at,omitempty"`
}

func (OrderItem) TableName() string {
	return "order_items"
}
