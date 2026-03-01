package routes

import (
	clientCtrl "api/app/controllers/client"
	clientAuth "api/app/controllers/client/auth"

	"api/pkg/config"

	"api/app/middlewares"

	"github.com/gin-gonic/gin"
)

// RegisterApiRoutes 注册网页相关路由
func RegisterApiRoutes(r *gin.Engine) {

	// 测试一个 v1 的路由组，我们所有的 v1 版本的路由都将存放到这里
	var v1 *gin.RouterGroup

	if len(config.Get[string]("app.api_domain")) == 0 {
		v1 = r.Group("/api")
	} else {
		v1 = r.Group("/v1")
	}

	v1.Use(middlewares.Common(), middlewares.LimitIP("200-H"))

	sys := new(clientCtrl.SystemController)
	systemGroup := v1.Group("/system")
	systemGroup.Use(middlewares.LimitIP("1000-H"))
	{
		systemGroup.GET("/health", sys.Health)
		systemGroup.POST("/task/push", sys.PushTask)
		systemGroup.POST("/deepseek/test", sys.DeepSeekTest)
	}

	videoCtrl := new(clientCtrl.VideoController)
	videoGroup := v1.Group("/video")
	videoGroup.Use(middlewares.LimitIP("300-H"))
	{
		videoGroup.POST("/content/idiom/create", videoCtrl.CreateIdiomStory)
		videoGroup.POST("/content/idiom/submit", videoCtrl.SubmitPlan)
	}

	authGroup := v1.Group("/auth")
	authGroup.Use(middlewares.LimitIP("1000-H"))
	{
		suc := new(clientAuth.SignupController)
		authGroup.POST("/signup/email/exist", suc.IsEmailExist)
		authGroup.POST("/signup/using-email", suc.SignupUsingEmail)

		sic := new(clientAuth.SigninController)
		authGroup.POST("/signin/using-password", sic.SignInByPassword)
		authGroup.POST("/signin/refresh_token", middlewares.AuthJWT(), sic.RefreshToken)

	}

}
