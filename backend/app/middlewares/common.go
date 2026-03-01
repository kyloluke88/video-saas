package middlewares

import (
	"github.com/gin-gonic/gin"
)

func Common() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := getLang(c)
		
		c.Set("lang", lang)

		c.Next()
	}
}

func getLang(c *gin.Context)string{
	lang := c.GetHeader("Accept-Language")
	if lang == "" {
		return "en-US"
	}
	return lang
}