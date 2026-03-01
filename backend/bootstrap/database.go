package bootstrap

import (
	"api/pkg/config"
	"api/pkg/database"
	"errors"
	"fmt"
	"time"

	// "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	// "gorm.io/driver/sqlite"
	"api/pkg/logger"

	"gorm.io/gorm"
)

// SetupDB 初始化数据库和 ORM
func SetupDB() {

	var dbConfig gorm.Dialector
	var poolPrefix string
	switch config.Get[string]("database.connection") {
	case "mysql":
		poolPrefix = "database.mysql"
		// 构建 DSN 信息
		// dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=%v&parseTime=True&multiStatements=true&loc=Local",
		//     config.Get[string]("database.mysql.username"),
		//     config.Get[string]("database.mysql.password"),
		//     config.Get[string]("database.mysql.host"),
		//     config.Get[string]("database.mysql.port"),
		//     config.Get[string]("database.mysql.database"),
		//     config.Get[string]("database.mysql.charset"),
		// )
		// dbConfig = mysql.New(mysql.Config{
		//     DSN: dsn,
		// })
	case "sqlite":
		poolPrefix = "database.sqlite"

		// 初始化 sqlite
		// database := config.Get[string]("database.sqlite.database")
		// dbConfig = sqlite.Open(database)
	case "postgresql":
		poolPrefix = "database.postgresql"

		dsn := fmt.Sprintf("host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
			config.Get[string]("database.postgresql.host"),
			config.Get[string]("database.postgresql.port"),
			config.Get[string]("database.postgresql.username"),
			config.Get[string]("database.postgresql.database"),
			config.Get[string]("database.postgresql.password"),
		)

		dbConfig = postgres.New(postgres.Config{DSN: dsn})

	default:
		panic(errors.New("database connection not supported"))
	}

	// 连接数据库，并设置 GORM 的日志模式
	database.Connect(dbConfig, logger.NewGormLogger())

	// 设置最大连接数
	database.SQLDB.SetMaxOpenConns(config.Get[int](poolPrefix + ".max_open_connections"))
	// 设置最大空闲连接数
	database.SQLDB.SetMaxIdleConns(config.Get[int](poolPrefix + ".max_idle_connections"))
	// 设置每个链接的过期时间
	database.SQLDB.SetConnMaxLifetime(time.Duration(config.Get[int](poolPrefix+".max_life_seconds")) * time.Second)
}
