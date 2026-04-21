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

	publicPodcastScriptCtrl := new(clientCtrl.PublicPodcastScriptController)
	publicProductCtrl := new(clientCtrl.PublicProductController)
	publicGroup := v1.Group("/public")
	publicGroup.Use(middlewares.LimitIP("1000-H"))
	{
		publicGroup.GET("/podcast/scripts", publicPodcastScriptCtrl.ListPages)
		publicGroup.GET("/podcast/scripts/:slug", publicPodcastScriptCtrl.ShowPage)
		publicGroup.GET("/products/:locale", publicProductCtrl.ListProducts)
		publicGroup.GET("/products/:locale/:slug", publicProductCtrl.ShowProduct)
	}

	analyticsCtrl := new(clientCtrl.AnalyticsController)
	analyticsGroup := v1.Group("/analytics")
	// 单独分组是为了把公开埋点的跨域策略和限流策略隔离开。
	analyticsGroup.Use(middlewares.PublicCORS(), middlewares.LimitIP("10000-H"))
	{
		analyticsGroup.OPTIONS("/page-view", analyticsCtrl.TrackPageView)
		analyticsGroup.POST("/page-view", analyticsCtrl.TrackPageView)
	}

	videoCtrl := new(clientCtrl.VideoController)
	videoGroup := v1.Group("/video")
	videoGroup.Use(middlewares.LimitIP("300-H"))
	{
		videoGroup.POST("/content/idiom/create", videoCtrl.CreateIdiomStory)
		videoGroup.POST("/content/idiom/submit", videoCtrl.SubmitPlan)
		videoGroup.POST("/content/podcast/create", videoCtrl.CreatePodcastDialogue)
		videoGroup.POST("/project/cancel", videoCtrl.CancelProject)
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
