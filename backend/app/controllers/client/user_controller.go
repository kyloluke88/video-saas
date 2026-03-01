package client

import (
	"api/pkg/response"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	BaseAPIController
}

func (ctrl UserController) CurrentUser(c *gin.Context) {
	response.JSON(c, gin.H{
		"data": "successful",
	})
}
