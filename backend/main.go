package main

import (
	btsConfig "api/config"
	"fmt"

	"api/bootstrap"

	"api/pkg/config"

	"api/app/cmd"
	"api/pkg/console"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	// 加载 config 目录下的配置信息
	btsConfig.Initialize()
}

func main() {

	// 配置初始化，依赖命令行 --env 参数
	// var env string
	// flag.StringVar(&env, "env", "", "加载 .env 文件，如 --env=testing 加载的是 .env.testing 文件")
	// flag.Parse()

	// config.InitConfig(env)

	// 初始化 Logger
	// bootstrap.SetupLogger()

	// 设置 gin 的运行模式，支持 debug, release, test
	// release 会屏蔽调试信息，官方建议生产环境中使用
	// 非 release 模式 gin 终端打印太多信息，干扰到我们程序中的 Log
	// 故此设置为 release，有特殊情况手动改为 debug 即可
	// gin.SetMode(gin.ReleaseMode) // excute in cmd package
	// 初始化gin

	// router := gin.Default() // excute in cmd package

	// bootstrap.SetupDB()
	// 初始化 Redis
	// bootstrap.SetupRedis()

	// bootstrap.SetupRoute(router) // excute in cmd package

	bootstrap.SetupCustomRules()

	// 运行服务
	// err := router.Run(":" + config.Get[string]("app.port"))
	// if err != nil {
	// 	// 错误处理，端口被占用了或者其他错误
	// 	fmt.Println(err.Error())
	// }

	// 应用的主入口，默认调用 cmd.CmdServe 命令
	var rootCmd = &cobra.Command{
		Use:   "api",
		Short: "A simple forum project",
		Long:  `Default will run "serve" command, you can use "-h" flag to see all subcommands`,

		// rootCmd 的所有子命令都会执行以下代码
		PersistentPreRun: func(command *cobra.Command, args []string) {

			// 配置初始化，依赖命令行 --env 参数
			config.InitConfig(cmd.Env)

			// 初始化 Logger
			bootstrap.SetupLogger()

			// 初始化数据库
			bootstrap.SetupDB()

			// 初始化 Redis
			bootstrap.SetupRedis()

			// 初始化 RabbitMQ
			bootstrap.SetupRabbitMQ()

			// 初始化缓存
		},
	}

	// 注册子命令
	rootCmd.AddCommand(
		cmd.CmdServe,
		cmd.CmdKey,
		cmd.CmdPlay,
	)

	// 配置默认运行 Web 服务
	cmd.RegisterDefaultCmd(rootCmd, cmd.CmdServe)

	// 注册全局参数，--env
	cmd.RegisterGlobalFlags(rootCmd)

	// 执行主命令
	if err := rootCmd.Execute(); err != nil {
		console.Exit(fmt.Sprintf("Failed to run app with %v: %s", os.Args, err.Error()))
	}
}
