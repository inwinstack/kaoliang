package models

import (
	"github.com/gocelery/gocelery"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var (
	celeryBroker  *gocelery.RedisCeleryBroker
	celeryBackend *gocelery.RedisCeleryBackend
)

func SetCelery() {
	celeryBroker = gocelery.NewRedisCeleryBroker("redis://" + utils.GetEnv("REDIS_ADDR", "localhost:6789"))
	celeryBackend = gocelery.NewRedisCeleryBackend("redis://" + utils.GetEnv("REDIS_ADDR", "localhost:6789"))
}

func GetCelery() (*gocelery.RedisCeleryBroker, *gocelery.RedisCeleryBackend) {
	return celeryBroker, celeryBackend
}
