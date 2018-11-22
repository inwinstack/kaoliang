package models

import (
	"github.com/gocelery/gocelery"
	"github.com/inwinstack/kaoliang/pkg/utils"
)

var (
	celeryBroker  *gocelery.RedisCeleryBroker
	celeryBackend *gocelery.RedisCeleryBackend
)

func SetCelery() {
	celeryBroker = gocelery.NewRedisCeleryBroker(utils.GetEnv("CELERY_BROKER_ADDR", "redis://localhost:6789"))
	celeryBackend = gocelery.NewRedisCeleryBackend(utils.GetEnv("CELERY_BACKEND_ADDR", "redis://localhost:6789"))
}

func GetCelery() (*gocelery.RedisCeleryBroker, *gocelery.RedisCeleryBackend) {
	return celeryBroker, celeryBackend
}
