package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func CreateQueue(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	queueName := c.Query("Name")
	db := models.GetDB()

	queue := models.Resource{
		Service:   models.SQS,
		AccountID: accountID,
		Name:      queueName,
	}

	db.Create(&queue)

	body := CreateQueueResponse{
		QueueURL:  queue.URL(),
		RequestID: "",
	}

	c.XML(http.StatusCreated, body)
}
