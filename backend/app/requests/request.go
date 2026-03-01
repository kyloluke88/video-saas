package requests

import (
	"api/pkg/i18n"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// TranslateValidationErrors
// 将 validator 错误转换为 i18n key，不做语言翻译
func TranslateValidationErrors(err error) map[string]string {
	result := make(map[string]string)

	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			// email.required / phone.phone_not_exists
			key := e.Field() + "." + e.Tag()
			result[e.Field()] = key
		}
	}

	return result
}

func MakeErrorMsg(c *gin.Context, err error) (errors map[string]string) {
	keys := TranslateValidationErrors(err)
	lang, _ := c.Get("lang")
	errors = make(map[string]string)
	for field, key := range keys {
		errors[field] = i18n.T(lang.(string), key)
	}
	return errors
}

// HandleBindError 根据错误类型统一返回绑定/验证错误，简化控制器代码。
func HandleBindError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	if _, ok := err.(validator.ValidationErrors); ok {
		response.ValidationError(c, MakeErrorMsg(c, err))
		return
	}

	response.BadRequest(c, err, "request bind failed")
}
