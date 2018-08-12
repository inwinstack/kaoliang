package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/satori/go.uuid"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
)

func CreateTopic(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	topicName := c.PostForm("Name")
	db := models.GetDB()

	topic := models.Resource{}
	db.Where(models.Resource{
		Service:   models.SNS,
		AccountID: accountID,
		Name:      topicName,
	}).FirstOrCreate(&topic)

	body := CreateTopicResponse{
		TopicARN:  topic.ARN(),
		RequestID: "",
	}
	c.XML(http.StatusOK, body)
}

func ListTopics(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	db := models.GetDB()
	topics := []models.Resource{}
	db.Where(&models.Resource{
		Service:   models.SNS,
		AccountID: accountID,
	}).Find(&topics)

	topicARNs := []TopicARN{}
	for _, topic := range topics {
		topicARNs = append(topicARNs, TopicARN{Name: topic.ARN()})
	}

	body := ListTopicsResponse{
		TopicARNs: topicARNs,
		RequestID: "",
	}

	c.XML(http.StatusOK, body)
}

func DeleteTopic(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	topicARN := c.PostForm("TopicArn")
	targetTopic, _ := models.ParseARN(topicARN)

	db := models.GetDB()
	topic := models.Resource{}

	db.Where(models.Resource{
		Service:   models.SNS,
		AccountID: accountID,
		Name:      targetTopic.Name,
	}).First(&topic)

	db.Delete(&topic)

	body := DeleteTopicResponse{
		RequestID: "",
	}

	c.XML(http.StatusOK, body)
}

func Subscribe(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	endpointURI := c.PostForm("Endpoint")
	protocol := c.PostForm("Protocol")
	topicARN := c.PostForm("TopicArn")
	targetTopic, _ := models.ParseARN(topicARN)
	targetTopic.AccountID = accountID

	db := models.GetDB()
	topic := models.Resource{}
	db.Where(targetTopic).First(&topic)

	endpointID, _ := uuid.NewV4()
	db.Model(&topic).Association("Endpoints").Append(models.Endpoint{
		Protocol: protocol,
		URI:      endpointURI,
		Name:     endpointID.String(),
	})

	RequestID, _ := uuid.NewV4()
	body := SubscribeResponse{
		SubscriptionARN: topic.ARN() + "/" + endpointID.String(),
		RequestID:       RequestID.String(),
	}

	c.XML(http.StatusOK, body)
}
