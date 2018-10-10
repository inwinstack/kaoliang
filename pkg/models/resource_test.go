package models_test

import (
	"testing"

	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/models"

	. "github.com/smartystreets/goconvey/convey"
)

func setup() {
	config.SetServerConfig()
}

func TestARN(t *testing.T) {
	setup()

	Convey("Given a topic instance", t, func() {
		topic := models.Resource{
			Service:   models.SNS,
			AccountID: "tester",
			Name:      "foobar",
		}

		Convey("When call ARN method", func() {
			arn := topic.ARN()

			Convey("Return value should equal to ARN that we defined", func() {
				So(arn, ShouldEqual, "arn:aws:sns:us-east-1:tester:foobar")
			})
		})
	})
}
