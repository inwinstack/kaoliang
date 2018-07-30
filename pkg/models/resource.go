package models

import "github.com/jinzhu/gorm"

type Service int

const (
	SQS Service = iota + 1
	SNS
)

type Resource struct {
	gorm.Model
	Service
	AccountID string
	Type      string
	Name      string
}
