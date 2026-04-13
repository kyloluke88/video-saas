package commerce

import (
	"time"

	"api/app/models"
)

type User struct {
	models.BaseModel

	Email          string     `gorm:"column:email;size:255;not null;uniqueIndex:uk_users_email" json:"email"`
	PasswordHash   *string    `gorm:"column:password_hash;type:text" json:"password_hash,omitempty"`
	DisplayName    *string    `gorm:"column:display_name;size:120" json:"display_name,omitempty"`
	Status         string     `gorm:"column:status;size:50;not null;default:active;index:idx_users_status" json:"status"`
	AuthProvider   *string    `gorm:"column:auth_provider;size:50;default:email" json:"auth_provider,omitempty"`
	ProviderUserID *string    `gorm:"column:provider_user_id;size:255" json:"provider_user_id,omitempty"`
	LastLoginAt    *time.Time `gorm:"column:last_login_at" json:"last_login_at,omitempty"`

	models.CommonTimestampsField
}

func (User) TableName() string {
	return "users"
}
