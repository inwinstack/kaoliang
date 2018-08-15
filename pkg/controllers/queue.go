package controllers

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/satori/go.uuid"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func ListQueues(c *gin.Context) {
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
		body := ErrorResponse{
			Type:      "Sender",
			Code:      "QueueAlreadyExists",
			Message:   "A queue with this name already exists.",
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
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	accountID = c.Param("account_id")
	queueName := c.Param("queue_name")

	db := models.GetDB()
	queue := models.Resource{}

	err := db.Where(models.Resource{Service: models.SQS, AccountID: accountID, Name: queueName}).First(&queue).Error
	if err != nil {
		c.XML(http.StatusOK, nil)
	}

	maxMsgNum, err := strconv.Atoi(c.Query("MaxNumberOfMessages"))
	if err != nil || maxMsgNum <= 0 {
		maxMsgNum = 1
	}
	if maxMsgNum > 10 {
		maxMsgNum = 10
	}

	redis := models.GetCache()
	key := fmt.Sprintf("sqs:%s:%s", accountID, queueName)
	bodys, _ := redis.LRange(key, 0, int64(maxMsgNum-1)).Result()
	redis.LTrim(key, int64(maxMsgNum), -1)

	msgs := []Message{}
	for _, body := range bodys {
		bodyMd5 := md5.Sum([]byte(body))
		msgId, _ := uuid.NewV4()
		receiptHandle := ""

		msg := Message{
			MessageID:     msgId.String(),
			ReceiptHandle: receiptHandle,
			Body:          body,
			MD5OfBody:     string(bodyMd5[:]),
		}
		msgs = append(msgs, msg)
	}

	requestID, _ := uuid.NewV4()
	response := ReceiveMessageResponse{
		Messages:  msgs,
		RequestID: requestID.String(),
	}
	c.XML(http.StatusOK, response)
}
