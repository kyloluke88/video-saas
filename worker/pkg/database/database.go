package database

import (
	"database/sql"

	"gorm.io/gorm"
)

var DB *gorm.DB
var SQLDB *sql.DB

func Set(db *gorm.DB, sqlDB *sql.DB) {
	DB = db
	SQLDB = sqlDB
}

func Ready() bool {
	return DB != nil && SQLDB != nil
}
