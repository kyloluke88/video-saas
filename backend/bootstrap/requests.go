package bootstrap

import (
	authRequest "api/app/requests/client/auth"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// SetupCustomRules 使用 Gin 默认 validator，并注册业务自定义规则。
func SetupCustomRules() {
	validate, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	// 显式注册 request 模块的自定义校验
	authRequest.RegisterEmailIsExist(validate)
}
