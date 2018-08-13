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
}

func writeErrorResponse(c *gin.Context, errorCode cmd.APIErrorCode) {
	apiError := cmd.GetAPIError(errorCode)
	errorResponse := cmd.GetAPIErrorResponse(apiError, c.Request.URL.Path)
	c.XML(apiError.HTTPStatusCode, errorResponse)
}
