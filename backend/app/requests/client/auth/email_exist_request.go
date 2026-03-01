package auth

import (
	"api/app/models/user"

	"github.com/go-playground/validator/v10"
)

type EmailIsExistRequest struct {
	Email string `json:"email" binding:"required,email,email_not_exists"`
}

func RegisterEmailIsExist(v *validator.Validate) {
	_ = v.RegisterValidation("email_not_exists", emailNotExists)
}

func emailNotExists(fl validator.FieldLevel) bool {
	// 返回false 验证失败
	return !user.IsEmailExist(fl.Field().String())
}
