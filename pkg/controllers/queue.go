package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func ListQueues(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	db := models.GetDB()
	var queues []models.Resource
	db.Where(&models.Resource{AccountID: accountID}).Find(&queues)

	body := ListQueuesResponse{}
	for _, queue := range queues {
		body.QueueURLs = append(body.QueueURLs, queue.URL())
	}

	c.XML(http.StatusOK, body)
}

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

func DeleteQueue(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	db := models.GetDB()
	queueName := c.Query("Name")
	queue := models.Resource{}

	db.Where(models.Resource{Service: models.SQS, AccountID: accountID, Name: queueName}).First(&queue)

	db.Delete(&queue)

	body := DeleteQueueResponse{
		RequestID: "",
	}

	c.XML(http.StatusOK, body)
}
