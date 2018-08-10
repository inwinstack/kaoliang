package controllers

import (
	"crypto/md5"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/satori/go.uuid"

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

	queueUrls := []string{}
	for _, queue := range queues {
		queueUrls = append(queueUrls, queue.URL())
	}

	requestID, _ := uuid.NewV4()
	body := ListQueuesResponse{
		QueueURLs: queueUrls,
		RequestID: requestID.String(),
	}

	c.XML(http.StatusOK, body)
}

func CreateQueue(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	queueName := c.Query("QueueName")
	db := models.GetDB()

	queue := models.Resource{
		Service:   models.SQS,
		AccountID: accountID,
		Name:      queueName,
	}

	requestID, _ := uuid.NewV4()

	// Response Error when queue is exists
	if !db.Where(&queue).First(&models.Resource{}).RecordNotFound() {
		body := ErrorResponse {
			Type: "Sender",
			Code: "QueueAlreadyExists",
			Message: "A queue with this name already exists.",
			RequestID: requestID.String(),
		}
		c.XML(http.StatusBadRequest, body)
		return
	}

	db.Create(&queue)
	body := CreateQueueResponse{
		QueueURL:  queue.URL(),
		RequestID: requestID.String(),
	}

	c.XML(http.StatusOK, body)
}

func DeleteQueue(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	accountID = c.Param("account_id")
	queueName := c.Param("queue_name")

	db := models.GetDB()
	queue := models.Resource{}

	db.Where(models.Resource{Service: models.SQS, AccountID: accountID, Name: queueName}).First(&queue)

	db.Delete(&queue)

	requestID, _ := uuid.NewV4()
	body := DeleteQueueResponse{
		RequestID: requestID.String(),
	}

	c.XML(http.StatusOK, body)
}

func ReceiveMessage(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	accountID = c.Param("account_id")
	queueName := c.Param("queue_name")

	db := models.GetDB()
	queue := models.Resource{}

	err = db.Where(models.Resource{Service: models.SQS, AccountID: accountID, Name: queueName}).First(&queue).Error
	if err != nil {
		c.XML(http.StatusOK, nil)
	}

	redis := models.GetCache()
	key := fmt.Sprintf("sqs:%s:%s", accountID, queueName)

	body, _ := redis.RPop(key).Result()
	body_md5 := md5.Sum([]byte(body))
	msg_id, _ := uuid.NewV4()
	receipt_handle := ""

	msg := Message{
		MessageID:     msg_id.String(),
		ReceiptHandle: receipt_handle,
		Body:          body,
		MD5OfBody:     string(body_md5[:]),
	}
	msgs := []Message{msg}

	requestID, _ := uuid.NewV4()
	response := ReceiveMessageResponse{
		Messages:  msgs,
		RequestID: requestID.String(),
	}
	c.XML(http.StatusOK, response)
}
