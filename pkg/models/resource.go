/*
Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package models

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/minio/minio/pkg/event"

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
	Endpoints []Endpoint
}

func (r Resource) URL() string {
	config := config.GetServerConfig()

	return fmt.Sprintf("http://%s/%s/%s", config.Host, r.AccountID, r.Name)
}

func (r Resource) ARN() string {
	config := config.GetServerConfig()

	return fmt.Sprintf("arn:aws:%s:%s:%s:%s", r.Service, config.Region, r.AccountID, r.Name)
}

func ParseService(s string) Service {
	services := map[string]Service{
		"sqs": SQS,
		"sns": SNS,
	}

	return services[s]
}

func ParseARN(s string) (*Resource, error) {
	if !strings.HasPrefix(s, "arn:aws:sqs") && !strings.HasPrefix(s, "arn:aws:sns") {
		return nil, &event.ErrInvalidARN{s}
	}

	tokens := strings.Split(s, ":")
	if len(tokens) != 6 && len(tokens) != 7 {
		return nil, &event.ErrInvalidARN{s}
	}

	if tokens[4] == "" || tokens[5] == "" {
		return nil, &event.ErrInvalidARN{s}
	}

	return &Resource{
		Service:   ParseService(tokens[2]),
		AccountID: tokens[4],
		Name:      tokens[5],
	}, nil
}
