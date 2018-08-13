package models

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var db *gorm.DB

func SetDB() {
	var err error
	db, err = gorm.Open("mysql", utils.GetEnv("DATABASE_URL", ""))
	if err != nil {
		panic(err)
	}
}

func Migrate() {
	db.AutoMigrate(&Resource{}, &Endpoint{})
}

func GetDB() *gorm.DB {
	return db
}
