package admin

import (
	"api/pkg/database"
)

func Get(idStr string)(adminModel Admin) {
	database.DB.Where("id", idStr).First(&adminModel)
	return
}