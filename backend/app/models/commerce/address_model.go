package commerce

import "api/app/models"

type Address struct {
	models.BaseModel

	UserID        uint64  `gorm:"column:user_id;not null;index:idx_addresses_user_id" json:"user_id"`
	RecipientName string  `gorm:"column:recipient_name;size:120;not null" json:"recipient_name"`
	Phone         *string `gorm:"column:phone;size:50" json:"phone,omitempty"`
	CountryCode   string  `gorm:"column:country_code;size:2;not null" json:"country_code"`
	State         *string `gorm:"column:state;size:120" json:"state,omitempty"`
	City          *string `gorm:"column:city;size:120" json:"city,omitempty"`
	District      *string `gorm:"column:district;size:120" json:"district,omitempty"`
	AddressLine1  string  `gorm:"column:address_line1;size:255;not null" json:"address_line1"`
	AddressLine2  *string `gorm:"column:address_line2;size:255" json:"address_line2,omitempty"`
	PostalCode    *string `gorm:"column:postal_code;size:50" json:"postal_code,omitempty"`
	IsDefault     bool    `gorm:"column:is_default;not null;default:false;index:idx_addresses_is_default" json:"is_default"`

	models.CommonTimestampsField
}

func (Address) TableName() string {
	return "addresses"
}
