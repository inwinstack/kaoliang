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

func writeErrorResponse(c *gin.Context, errorCode cmd.APIErrorCode) {
	apiError := cmd.GetAPIError(errorCode)
	errorResponse := cmd.GetAPIErrorResponse(apiError, c.Request.URL.Path)
	c.XML(apiError.HTTPStatusCode, errorResponse)
}
