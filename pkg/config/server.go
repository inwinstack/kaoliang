package config

import (
	"net/http"

	"github.com/minio/minio/cmd"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var serverConfig *ServerConfig

type ServerConfig struct {
	Region      string
	Host        string
	AuthBackend AuthenticationBackend
}

func SetServerConfig() {
	serverConfig = &ServerConfig{
		Region:      utils.GetEnv("RGW_REGION", "us-east-1"),
		Host:        utils.GetEnv("RGW_DNS_NAME", "cloud.inwinstack.com"),
		AuthBackend: SetAuthBackend(utils.GetEnv("AUTH_BACKEND", "DummyBackend")),
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
	userId, err := cmd.ReqSignatureV4Verify(r, "us-east-1")
	return userId, err
}

func SetAuthBackend(backend string) AuthenticationBackend {
	backends := map[string]AuthenticationBackend{
		"DummyBackend": DummyBackend{},
		"CephBackend":  CephBackend{},
	}

	return backends[backend]
}
