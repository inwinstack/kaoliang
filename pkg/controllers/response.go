package controllers

import (
	"encoding/xml"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
)

type ListQueuesResponse struct {
	XMLName   xml.Name `xml:"ListQueuesResponse"`
	QueueURLs []string `xml:"ListQueuesResult>QueueUrl"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

type CreateQueueResponse struct {
	XMLName   xml.Name `xml:"CreateQueueResponse"`
	QueueURL  string   `xml:"CreateQueueResult>QueueUrl"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

type DeleteQueueResponse struct {
	XMLName   xml.Name `xml:"DeleteQueueResponse"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

type ReceiveMessageResponse struct {
	XMLName   xml.Name  `xml:"ReceiveMessageResponse"`
	Messages  []Message `xml:"ReceiveMessageResult"`
	RequestID string    `xml:"ResponseMetadata>RequestId"`
}

type Message struct {
	XMLName       xml.Name `xml:"Message"`
	MessageID     string   `xml:"MessageId"`
	ReceiptHandle string   `xml:"ReceiptHandle"`
	MD5OfBody     string   `xml:"MD5OfBody"`
	Body          string   `xml:"Body"`
}

type ErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Type      string   `xml:"Error>Type"`
	Code      string   `xml:"Error>Code"`
	Message   string   `xml:"Error>Message"`
	RequestID string   `xml:"RequestId"`
type CreateTopicResponse struct {
	XMLName   xml.Name `xml:"CreateTopicResponse"`
	TopicARN  string   `xml:"CreateTopicResult>TopicArn"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

type TopicARN struct {
	Name string `xml:"TopicArn"`
}

type ListTopicsResponse struct {
	XMLName   xml.Name   `xml:"ListTopicsResponse"`
	TopicARNs []TopicARN `xml:"ListTopicsResult>Topics>member"`
	RequestID string     `xml:"ResponseMetadata>RequestId"`
}

type DeleteTopicResponse struct {
	XMLName   xml.Name `xml:"DeleteTopicResponse"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

type SubscribeResponse struct {
	XMLName         xml.Name `xml:"SubscribeResponse"`
	SubscriptionARN string   `xml:"SubscribeResult>SubscriptionArn"`
	RequestID       string   `xml:"ResponseMetadata>RequestId"`
}

type SubscriptionARN struct {
	TopicARN string `xml:"TopicArn"`
	Protocol string `xml:"Protocol"`
	ARN      string `xml:"SubscriptionArn"`
	Owner    string `xml:"Owner"`
}

type ListSubscriptionsResponse struct {
	XMLName          xml.Name          `xml:"ListSubscriptionsResponse"`
	SubscriptionARNs []SubscriptionARN `xml:"ListSubscriptionsResult>Subscriptions>member"`
	RequestID        string            `xml:"ResponseMetadata>RequestId"`
}

type UnsubscribeResponse struct {
	XMLName   xml.Name `xml:"UnsubscribeResponse"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

func writeErrorResponse(c *gin.Context, errorCode cmd.APIErrorCode) {
	apiError := cmd.GetAPIError(errorCode)
	errorResponse := cmd.GetAPIErrorResponse(apiError, c.Request.URL.Path)
	c.XML(apiError.HTTPStatusCode, errorResponse)
}
