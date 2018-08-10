package controllers

import (
	"net/http"

	"github.com/minio/minio/cmd"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/config"
)

func authenticate(r *http.Request) (string, cmd.APIErrorCode) {
	config := config.GetServerConfig()
	return config.AuthBackend.GetUser(r)
}
