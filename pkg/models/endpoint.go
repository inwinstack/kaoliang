package models

import "github.com/jinzhu/gorm"

type Endpoint struct {
	gorm.Model
	Protocol   string
	URI        string
	Name       string
	ResourceID uint
}
