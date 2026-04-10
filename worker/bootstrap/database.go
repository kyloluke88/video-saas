package bootstrap

import (
	"errors"
	"fmt"
	"time"

	conf "worker/pkg/config"
	"worker/pkg/database"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupDB() error {
	if conf.Get[string]("worker.db_connection") != "postgresql" {
		return errors.New("database connection not supported")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		conf.Get[string]("worker.db_host"),
		conf.Get[string]("worker.db_port"),
		conf.Get[string]("worker.db_username"),
		conf.Get[string]("worker.db_database"),
		conf.Get[string]("worker.db_password"),
		conf.Get[string]("worker.db_sslmode"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(conf.Get[int]("worker.db_max_idle_connections"))
	sqlDB.SetMaxOpenConns(conf.Get[int]("worker.db_max_open_connections"))
	sqlDB.SetConnMaxLifetime(time.Duration(conf.Get[int]("worker.db_max_life_seconds")) * time.Second)

	database.Set(db, sqlDB)
	return nil
}
