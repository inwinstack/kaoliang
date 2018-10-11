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
