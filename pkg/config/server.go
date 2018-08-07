package config

import (
	"net/http"

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
	GetUser(*http.Request) (string, error)
}

type DummyBackend struct {
}

func (b DummyBackend) GetUser(r *http.Request) (string, error) {
	return "tester", nil
}

func SetAuthBackend(backend string) AuthenticationBackend {
	backends := map[string]AuthenticationBackend{
		"DummyBackend": DummyBackend{},
	}

	return backends[backend]
}
