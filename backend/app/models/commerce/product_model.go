package commerce

import (
	"encoding/json"

	"api/app/models"
)

type Product struct {
	models.BaseModel

	ProductCode   string          `gorm:"column:product_code;size:100;not null;uniqueIndex:uk_products_product_code" json:"product_code"`
	Name          string          `gorm:"column:name;size:255;not null" json:"name"`
	Locale        string          `gorm:"column:locale;size:8;not null;default:zh;index:idx_products_locale;uniqueIndex:uk_products_locale_slug,priority:1" json:"locale"`
	Slug          string          `gorm:"column:slug;size:255;not null;uniqueIndex:uk_products_locale_slug,priority:2" json:"slug"`
	ProductType   string          `gorm:"column:product_type;size:50;not null;index:idx_products_product_type" json:"product_type"`
	Status        string          `gorm:"column:status;size:50;not null;default:draft;index:idx_products_status" json:"status"`
	MinPrice      *float64        `gorm:"column:min_price;type:numeric(12,2)" json:"min_price,omitempty"`
	MaxPrice      *float64        `gorm:"column:max_price;type:numeric(12,2)" json:"max_price,omitempty"`
	Currency      string          `gorm:"column:currency;size:3;not null;default:USD" json:"currency"`
	IsVirtual     bool            `gorm:"column:is_virtual;not null;default:true;index:idx_products_is_virtual" json:"is_virtual"`
	CoverImageURL *string         `gorm:"column:cover_image_url;type:text" json:"cover_image_url,omitempty"`
	Description   *string         `gorm:"column:description;type:text" json:"description,omitempty"`
	Metadata      json.RawMessage `gorm:"column:metadata;type:jsonb;not null;default:'{}'" json:"metadata,omitempty"`

	models.CommonTimestampsField
}

func (Product) TableName() string {
	return "products"
}
