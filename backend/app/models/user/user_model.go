// Package user 存放用户 Model 相关逻辑
package user

import (
	"api/app/models"
	"api/pkg/database"
	"api/pkg/hash"

	// "gohub/pkg/hash"
	"fmt"
)

// User 用户模型
type User struct {
	models.BaseModel
	FirstName   string  `json:"first_name,omitempty"`
	LastName    string  `json:"last_name,omitempty"`
	DisplayName string  `json:"display_name,omitempty" gorm:"column:display_name;<-:false"`
	Email       string  `json:"email"`
	Phone       *string `json:"phone"` // 因为零值为 “”， 会触发DB的phone unique constraint Error， 所以这里使用指针实现可有可无
	Password    string  `json:"-"`
	Avatar      string  `json:"avatar"`

	models.CommonTimestampsField
}

func (User) TableName() string {
	return "users"
}

// Create 创建用户，通过 User.ID 来判断是否创建成功
func (userModel *User) Create() {
	database.DB.Create(&userModel)
}

// ComparePassword 密码是否正确
func (userModel *User) ComparePassword(_password string) bool {
	return hash.BcryptCheck(_password, userModel.Password)
}

func (userModel *User) Save() (rowsAffected int64) {
	result := database.DB.Save(&userModel)
	return result.RowsAffected
}

func (userModel *User) FullName() string {
	return fmt.Sprintf("%s %s", userModel.FirstName, userModel.LastName)
}
