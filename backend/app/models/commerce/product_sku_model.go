package commerce

import (
	"encoding/json"

	"api/app/models"
)

type ProductSKU struct {
	models.BaseModel

	ProductID     uint64          `gorm:"column:product_id;not null;index:idx_product_skus_product_id" json:"product_id"`
	SKUCode       string          `gorm:"column:sku_code;size:100;not null;uniqueIndex:uk_product_skus_sku_code" json:"sku_code"`
	Name          string          `gorm:"column:name;size:255;not null" json:"name"`
	Price         float64         `gorm:"column:price;type:numeric(12,2);not null;index:idx_product_skus_price" json:"price"`
	OriginalPrice *float64        `gorm:"column:original_price;type:numeric(12,2)" json:"original_price,omitempty"`
	Currency      string          `gorm:"column:currency;size:3;not null;default:USD" json:"currency"`
	Status        string          `gorm:"column:status;size:50;not null;default:active;index:idx_product_skus_status" json:"status"`
	IsDefault     bool            `gorm:"column:is_default;not null;default:false;index:idx_product_skus_is_default" json:"is_default"`
	IsVirtual     bool            `gorm:"column:is_virtual;not null;default:true;index:idx_product_skus_is_virtual" json:"is_virtual"`
	StockQty      *int            `gorm:"column:stock_qty" json:"stock_qty,omitempty"`
	Attributes    json.RawMessage `gorm:"column:attributes;type:jsonb;not null;default:'{}'" json:"attributes,omitempty"`

	models.CommonTimestampsField
}

func (ProductSKU) TableName() string {
	return "product_skus"
}
