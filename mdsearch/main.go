package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/inwinstack/kaoliang/pkg/caches"
	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/controllers"
	"github.com/inwinstack/kaoliang/pkg/models"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	config.SetServerConfig()
	models.SetElasticsearch()
	caches.SetRedis()
}

func main() {
	r := gin.Default()
	r.RedirectTrailingSlash = false

	r.GET("/:bucket/", controllers.Search)

	r.Run()
}
