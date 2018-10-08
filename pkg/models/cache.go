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
	"github.com/go-redis/redis"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var client *redis.Client

func SetCache() {
	client = redis.NewClient(&redis.Options{
		Addr:     utils.GetEnv("REDIS_ADDR", "localhost:6789"),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

func GetCache() *redis.Client {
	return client
}
