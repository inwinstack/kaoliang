/*
Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

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
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
		c.Header("Access-Control-Allow-Headers", "x-amz-content-sha256,x-amz-date,authorization,host,x-amz-user-agent")

		c.Status(http.StatusNoContent)
	})

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

	r.POST("/", func(c *gin.Context) {
		action := c.PostForm("Action")
		switch action {
		case "ListQueues":
			controllers.ListQueues(c)
		case "CreateQueue":
			controllers.CreateQueue(c)
		case "DeleteQueue":
			controllers.DeleteQueue(c)
		case "ReceiveMessage":
			controllers.ReceiveMessage(c)
		}
	})

	r.Run()
}
