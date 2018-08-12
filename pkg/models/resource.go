package models

import (
	"database/sql/driver"
	"fmt"

	"github.com/jinzhu/gorm"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/config"
)

type Service int

const (
	SQS Service = iota + 1
	SNS
)

func (s Service) String() string {
	services := map[Service]string{
		SQS: "sqs",
		SNS: "sns",
	}

	return services[s]
}

func (s *Service) Scan(src interface{}) error {
	switch src.(type) {
	case int64:
		*s = Service(src.(int64))
	}

	return nil
}

func (s Service) Value() (driver.Value, error) {
	return int64(s), nil
}

type Resource struct {
	gorm.Model
	Service
	AccountID string
	Type      string
	Name      string
}

func (r Resource) URL() string {
	config := config.GetServerConfig()

	return fmt.Sprintf("http://%s/%s/%s", config.Host, r.AccountID, r.Name)
}
