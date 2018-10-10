package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/controllers"
	"github.com/inwinstack/kaoliang/pkg/models"
)

func setup() {
	os.Setenv("RGW_DNS_NAME", "cloud.inwinstack.com")
	os.Setenv("DATABASE_URL", "root:my-secret-pw@tcp(127.0.0.1:3306)/test_kaoliang?charset=utf8&parseTime=True&loc=Local")
	config.SetServerConfig()
	models.SetDB()
	models.Migrate()
}

func teardown() {
	db := models.GetDB()
	db.Exec("TRUNCATE TABLE resources;")
}

func TestListQueues(t *testing.T) {
	setup()
	defer teardown()

	Convey("Given some resource records", t, func() {
		db := models.GetDB()
		var queue models.Resource

		for _, name := range []string{"gin", "kaoliang"} {
			queue = models.Resource{
				Service:   models.SQS,
				AccountID: "tester",
				Name:      name,
			}

			db.Create(&queue)
		}

		Convey("When access to list queues controller", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/?Action=ListQueues", nil)
			controllers.ListQueues(c)

			Convey("The status code of response should equal to 200", func() {
				So(w.Code, ShouldEqual, 200)
			})
		})
	})
}

func TestCreateQueue(t *testing.T) {
	setup()
	defer teardown()

	Convey("Given a create queue request", t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/?Action=CreateQueue&Name=kaoliang", nil)

		Convey("When send it to create queue controller", func() {
			controllers.CreateQueue(c)

			Convey("The status code of response should equal to 201", func() {
				So(w.Code, ShouldEqual, 201)
			})
		})
	})
}

func TestDeleteQueue(t *testing.T) {
	setup()
	defer teardown()

	Convey("Given a queue", t, func() {
		db := models.GetDB()
		queue := models.Resource{
			Service:   models.SQS,
			AccountID: "tester",
			Name:      "kaoliang",
		}
		db.Create(&queue)

		Convey("When access to delete queue controller", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/?Action=DeleteQueue&Name=kaoliang", nil)
			controllers.DeleteQueue(c)

			Convey("The status code of response should equal to 200", func() {
				So(w.Code, ShouldEqual, 200)
			})
		})
	})
}
