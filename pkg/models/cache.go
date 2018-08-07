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
