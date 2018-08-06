package controllers

import (
	"net/http"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/config"
)

func authenticate(r *http.Request) (string, error) {
	config := config.GetServerConfig()
	return config.AuthBackend.GetUser(r)
}
