package controllers

import (
	"crypto/md5"
	"fmt"
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

	queueName := c.Query("QueueName")
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

	body := DeleteQueueResponse{
		RequestID: "",
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
	msg_id := "5fea7756-0ea4-451a-a703-a558b933e274"
	receipt_handle := ""

	msg := Message{
		MessageID:     msg_id,
		ReceiptHandle: receipt_handle,
		Body:          body,
		MD5OfBody:     string(body_md5[:]),
	}
	msgs := []Message{msg}

	response := ReceiveMessageResponse{
		Messages:  msgs,
		RequestID: "",
	}
	c.XML(http.StatusOK, response)
}
