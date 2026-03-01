// Package auth 处理用户身份认证相关逻辑
package auth

import (
	client "api/app/controllers/client"
	"api/pkg/jwt"
	"api/pkg/logger"

	authRequest "api/app/requests/client/auth"
	"api/pkg/auth"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
)

// SigninController 注册控制器
type SigninController struct {
	client.BaseAPIController
}

func (sc *SigninController) SignInByPassword(c *gin.Context) {

	var req authRequest.SigninByPasswordRequest

	if !sc.BindJSON(c, &req) {
		return
	}

	logger.DebugJSON("login", "req", req)
	user, err := auth.Attempt(req.LoginId, req.Password)
	if err != nil {
		response.Unauthorized(c, "login_id is not exist or password is incorrect")
		return
	} else {
		token := jwt.NewJWT().IssueToken(user.GetStringID(), user.FullName(), "user", "shop-user")
		response.JSON(c, gin.H{
			"token": token,
		})
	}
}

func (sc *SigninController) RefreshToken(c *gin.Context) {

	token, err := jwt.NewJWT().RefreshToken(c)
	if err != nil {
		response.Error(c, err, "token refresh failed")
	} else {
		response.JSON(c, gin.H{
			"token": token,
		})
	}
}
