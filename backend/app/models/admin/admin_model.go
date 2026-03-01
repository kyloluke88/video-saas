package admin

import (
	"api/app/models"
	"api/pkg/database"
)

type Admin struct {
	models.BaseModel

	FirstName string `json:"first_name,omitempty"`
	LastName string `json:"last_name,omitempty"`
	City         string `json:"city,omitempty"`
	Avatar       string `json:"avatar,omitempty"`

	Email    string `json:"-"`
	Phone    string `json:"-"`
	Password string `json:"-"`

	models.CommonTimestampsField
}

func (Admin) TableName() string {
	return "admins"
}

// Create 创建用户，通过 User.ID 来判断是否创建成功
func (adminModel *Admin) Create() {
	database.DB.Create(&adminModel)
}

// ComparePassword 密码是否正确
// func (userModel *User) ComparePassword(_password string) bool {
// 	return hash.BcryptCheck(_password, userModel.Password)
// }

func (adminModel *Admin) Save() (rowsAffected int64) {
	result := database.DB.Save(&adminModel)
	return result.RowsAffected
}

