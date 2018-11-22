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

package config

import (
	"net/http"

	"github.com/minio/minio/cmd"

	"github.com/inwinstack/kaoliang/pkg/utils"
)

var serverConfig *ServerConfig

type ServerConfig struct {
	Region               string
	Host                 string
	AuthBackend          AuthenticationBackend
	Scheme               string
	EnableKaoliangCreate string
	EnableKaoliangCopy   string
	EnableKaoliangDelete string
}

func SetServerConfig() {
	serverConfig = &ServerConfig{
		Region:               utils.GetEnv("RGW_REGION", "us-east-1"),
		Host:                 utils.GetEnv("RGW_DNS_NAME", "cloud.inwinstack.com"),
		AuthBackend:          SetAuthBackend(utils.GetEnv("AUTH_BACKEND", "DummyBackend")),
		Scheme:               utils.GetEnv("SCHEME", "http"),
		EnableKaoliangCreate: utils.GetEnv("ENABLE_KAOLIANG_CREATE", "True"),
		EnableKaoliangCopy:   utils.GetEnv("ENABLE_KAOLIANG_COPY", "True"),
		EnableKaoliangDelete: utils.GetEnv("ENABLE_KAOLIANG_DELETE", "True"),
	}
}

func GetServerConfig() *ServerConfig {
	return serverConfig
}

type AuthenticationBackend interface {
	GetUser(*http.Request) (string, cmd.APIErrorCode)
}

type DummyBackend struct {
}

func (b DummyBackend) GetUser(r *http.Request) (string, cmd.APIErrorCode) {
	return "tester", cmd.ErrNone
}

type CephBackend struct {
}

func (b CephBackend) GetUser(r *http.Request) (string, cmd.APIErrorCode) {
	userId, err := cmd.ReqSignatureV4Verify(r, "")
	return userId, err
}

func SetAuthBackend(backend string) AuthenticationBackend {
	backends := map[string]AuthenticationBackend{
		"DummyBackend": DummyBackend{},
		"CephBackend":  CephBackend{},
	}

	return backends[backend]
}
