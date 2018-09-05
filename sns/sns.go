package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/config"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/controllers"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	config.SetServerConfig()
	models.SetDB()
	models.Migrate()
	models.SetCache()
}

func setOriginHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
	}
}

func main() {
	r := gin.Default()
	r.Use(setOriginHeader())

	r.OPTIONS("/", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
		c.Header("Access-Control-Allow-Headers", "x-amz-content-sha256,x-amz-date,authorization,host,x-amz-user-agent")

		c.Status(http.StatusNoContent)
	})

	r.POST("/", func(c *gin.Context) {
		action := c.PostForm("Action")
		switch action {
		case "CreateTopic":
			controllers.CreateTopic(c)
		case "ListTopics":
			controllers.ListTopics(c)
		case "DeleteTopic":
			controllers.DeleteTopic(c)
		case "Subscribe":
			controllers.Subscribe(c)
		case "ListSubscriptions":
			controllers.ListSubscriptions(c)
		case "Unsubscribe":
			controllers.Unsubscribe(c)
		}
	})

	r.Run()
}
