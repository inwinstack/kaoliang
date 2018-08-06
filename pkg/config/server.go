package config

import (
	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var serverConfig *ServerConfig

type ServerConfig struct {
	Region string
	Host   string
}

func SetServerConfig() {
	serverConfig = &ServerConfig{
		Region: utils.GetEnv("RGW_REGION", "us-east-1"),
		Host:   utils.GetEnv("RGW_DNS_NAME", "cloud.inwinstack.com"),
	}
}

func GetServerConfig() *ServerConfig {
	return serverConfig
}
