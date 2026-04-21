package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PublicCORS 为公开埋点接口提供最小 CORS 支持。
// 这里不允许凭证，避免把公开分析接口和登录态耦合在一起。
func PublicCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 预检请求会先发 OPTIONS，浏览器用它确认跨域是否允许。
		// 真正的埋点数据仍然通过后续的 POST 发送。
		// 这里不返回 Allow-Credentials，因为埋点接口不依赖 cookie 登录态。
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Accept, Origin, X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
