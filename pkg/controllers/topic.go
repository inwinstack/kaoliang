/*
Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package controllers

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/satori/go.uuid"

	"github.com/inwinstack/kaoliang/pkg/models"
)

func CreateTopic(c *gin.Context) {
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	topicName := c.PostForm("Name")
	db := models.GetDB()

	requestID, _ := uuid.NewV4()
	re := regexp.MustCompile("^[\\w-]{1,256}$")
	if !re.MatchString(topicName) {
		body := ErrorResponse{
			Type:      "Sender",
			Code:      "InvalidParameter",
			Message:   "InvalidParameter: Topic Name",
			RequestID: requestID.String(),
		}
		c.XML(http.StatusBadRequest, body)
		return
	}

	topic := models.Resource{}
	db.Where(models.Resource{
		Service:   models.SNS,
		AccountID: accountID,
		Name:      topicName,
	}).FirstOrCreate(&topic)

	body := CreateTopicResponse{
		TopicARN:  topic.ARN(),
		RequestID: requestID.String(),
	}
	c.XML(http.StatusOK, body)
}

func ListTopics(c *gin.Context) {
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
	userID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	topicARN := c.PostForm("TopicArn")
	targetTopic, _ := models.ParseARN(topicARN)

	db := models.GetDB()
	topic := models.Resource{}

	if userID != targetTopic.AccountID {
		writeErrorResponse(c, cmd.ErrAuthorizationError)
		return
	}

	db.Where(models.Resource{
		Service:   models.SNS,
		AccountID: targetTopic.AccountID,
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
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
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
					Endpoint: endpoint.URI,
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

func Unsubscribe(c *gin.Context) {
	accountID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	subscriptionARN := c.PostForm("SubscriptionArn")
	targetTopic, _ := models.ParseARN(subscriptionARN)
	targetSubscription, err := models.ParseSubscription(subscriptionARN)
	requestID, _ := uuid.NewV4()
	if err != nil {
		body := ErrorResponse{
			Type:      "Sender",
			Code:      "InvalidParameter",
			Message:   "Invalid parameter: SubscriptionId",
			RequestID: requestID.String(),
		}
		c.XML(http.StatusBadRequest, body)
		return
	}

	topic := models.Resource{}
	subscription := models.Endpoint{}
	db := models.GetDB()
	targetTopic.AccountID = accountID
	db.Where(targetTopic).First(&topic)
	targetSubscription.ResourceID = topic.ID
	db.Where(targetSubscription).First(&subscription)

	db.Delete(&subscription)

	body := UnsubscribeResponse{
		RequestID: requestID.String(),
	}

	c.XML(http.StatusOK, body)
}
