package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/config"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/controllers"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func setup() {
	os.Setenv("RGW_DNS_NAME", "cloud.inwinstack.com")
	os.Setenv("DATABASE_URL", "root:my-secret-pw@tcp(127.0.0.1:3306)/test_kaoliang")
	config.SetServerConfig()
	models.SetDB()
	models.Migrate()
}

func teardown() {
	db := models.GetDB()
	db.Exec("TRUNCATE TABLE resources;")
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
