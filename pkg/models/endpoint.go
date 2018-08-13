package models

import (
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/minio/minio/pkg/event"
)

type Endpoint struct {
	gorm.Model
	Protocol   string
	URI        string
	Name       string
	ResourceID uint
}

func ParseSubscription(s string) (*Endpoint, error) {
	if _, err := ParseARN(s); err != nil {
		return nil, &event.ErrInvalidARN{s}
	}

	tokens := strings.Split(s, ":")

	return &Endpoint{
		Name: tokens[6],
	}, nil
}
