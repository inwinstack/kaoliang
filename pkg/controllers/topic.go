package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func CreateTopic(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	topicName := c.PostForm("Name")
	db := models.GetDB()

	topic := models.Resource{
		Service:   models.SNS,
		AccountID: accountID,
		Name:      topicName,
	}

	db.FirstOrCreate(&topic)
	body := CreateTopicResponse{
		TopicARN:  topic.ARN(),
		RequestID: "",
	}
	c.XML(http.StatusOK, body)
}
