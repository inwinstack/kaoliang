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

	requestID, _ := uuid.NewV4()
	body := CreateTopicResponse{
		TopicARN:  topic.ARN(),
		RequestID: requestID.String(),
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

	requestID, _ := uuid.NewV4()
	body := ListTopicsResponse{
		TopicARNs: topicARNs,
		RequestID: requestID.String(),
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

	requestID, _ := uuid.NewV4()
	body := DeleteTopicResponse{
		RequestID: requestID.String(),
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

	requestID, _ := uuid.NewV4()
	body := SubscribeResponse{
		SubscriptionARN: topic.ARN() + ":" + endpointID.String(),
		RequestID:       requestID.String(),
	}

	c.XML(http.StatusOK, body)
}

func ListSubscriptions(c *gin.Context) {
	accountID, err := authenticate(c.Request)
	if err != nil {
		writeErrorResponse(c, cmd.ToAPIErrorCode(err))
	}

	topics := []models.Resource{}
	db := models.GetDB()
	db.Where(models.Resource{AccountID: accountID}).Find(&topics)

	subscriptionARNs := []SubscriptionARN{}
	for _, topic := range topics {
		endpoints := []models.Endpoint{}
		db.Model(&topic).Association("Endpoints").Find(&endpoints)
		if len(endpoints) > 0 {
			for _, endpoint := range endpoints {
				subscriptionARNs = append(subscriptionARNs, SubscriptionARN{
					TopicARN: topic.ARN(),
					Protocol: endpoint.Protocol,
					ARN:      topic.ARN() + ":" + endpoint.Name,
					Owner:    topic.AccountID,
				})
			}
		}
	}

	requestID, _ := uuid.NewV4()
	body := ListSubscriptionsResponse{
		SubscriptionARNs: subscriptionARNs,
		RequestID:        requestID.String(),
	}
	c.XML(http.StatusOK, body)
}
