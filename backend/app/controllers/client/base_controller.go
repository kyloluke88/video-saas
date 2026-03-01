// Package client 处理业务逻辑
package client

import (
	"api/app/requests"

	"github.com/gin-gonic/gin"
)

// BaseAPIController 基础控制器
type BaseAPIController struct {
}

// BindJSON 统一处理绑定与验证错误，控制器只需关注业务逻辑。
func (bc *BaseAPIController) BindJSON(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		requests.HandleBindError(c, err)
		return false
	}
	return true
}
