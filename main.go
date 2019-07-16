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
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"syscall"

	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/inwinstack/kaoliang/pkg/caches"
	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/controllers"
	"github.com/inwinstack/kaoliang/pkg/models"
	"github.com/inwinstack/kaoliang/pkg/utils"
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
	models.SetCelery()
	caches.SetRedis()
}

func main() {
	r := gin.Default()
	r.RedirectTrailingSlash = false

	r.GET("/:bucket", controllers.GetBucketNotification)
	r.PUT("/:bucket", controllers.PutBucketNotification)
	r.PATCH("/:bucket", controllers.PatchBucketPermission)
	r.PATCH("/:bucket/", controllers.PatchBucketPermission)
	r.POST("/objects", controllers.MoveObjects)

	r.NoRoute(controllers.ReverseProxy())

	server := endless.NewServer(fmt.Sprintf(":%s", utils.GetEnv("PORT", "8003")), r)
	server.BeforeBegin = func(add string) {
		pid := syscall.Getpid()
		err := ioutil.WriteFile("/var/run/kaoliang/s3.pid", []byte(strconv.Itoa(pid)), 0644)
		if err != nil {
			panic(err)
		}
	}

	server.ListenAndServe()
}
