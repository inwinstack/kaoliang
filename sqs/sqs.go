package main

import (
	"log"

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

func main() {
	r := gin.Default()

	r.GET("/:account_id/:queue_name", func(c *gin.Context) {
		action := c.Query("Action")
		switch action {
		case "DeleteQueue":
			controllers.DeleteQueue(c)
		case "ReceiveMessage":
			controllers.ReceiveMessage(c)
		}
	})

	r.GET("/", func(c *gin.Context) {
		action := c.Query("Action")
		switch action {
		case "ListQueues":
			controllers.ListQueues(c)
		case "CreateQueue":
			controllers.CreateQueue(c)
		}
	})

	r.Run(":8888")
}
