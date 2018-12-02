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
	Messages  []Message `xml:"ReceiveMessageResult>Message"`
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
	XMLName   xml.Name `xml:"ErrorResponse" json:"-"`
	Type      string   `xml:"Error>Type"`
	Code      string   `xml:"Error>Code"`
	Message   string   `xml:"Error>Message"`
	RequestID string   `xml:"RequestId"`
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
	Endpoint string `xml:"Endpoint"`
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
