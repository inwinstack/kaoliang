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
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"

	sh "github.com/codeskyblue/go-sh"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/gocelery/gocelery"
	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/models"
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/event"
)

var errNoSuchNotifications = errors.New("The specified bucket does not have bucket notifications")

func PreflightRequest(c *gin.Context) {
	_, notification := c.GetQuery("notification")

	if notification {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
		c.Header("Access-Control-Allow-Headers", "content-type,x-amz-content-sha256,x-amz-date,authorization,host,x-amz-user-agent")

		c.Status(http.StatusNoContent)
	} else {
		ReverseProxy()(c)
	}
}

func GetBucketNotification(c *gin.Context) {
	userID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	bucket := c.Param("bucket")
	users, ok := getBucketUsers(bucket)
	if !ok {
		writeErrorResponse(c, cmd.ErrNoSuchBucket)
		return
	}

	if !contains(users, userID) {
		writeErrorResponse(c, cmd.ErrAccessDenied)
		return
	}

	if _, ok := c.GetQuery("notification"); ok {
		c.Header("Access-Control-Allow-Origin", "*")
		db := models.GetDB()
		nConfig := models.Config{}
		db.Where(&models.Config{Bucket: bucket}).
			Preload("Queues.Events").Preload("Queues.Resource").Preload("Queues.Filter.RuleList.Rules").
			Preload("Topics.Events").Preload("Topics.Resource").Preload("Topics.Filter.RuleList.Rules").
			First(&nConfig)
		c.XML(http.StatusOK, nConfig)
		return
	}

	ReverseProxy()(c)
}

func PutBucketNotification(c *gin.Context) {
	userID, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	bucket := c.Param("bucket")
	users, ok := getBucketUsers(bucket)
	if !ok {
		writeErrorResponse(c, cmd.ErrNoSuchBucket)
		return
	}

	if !contains(users, userID) {
		writeErrorResponse(c, cmd.ErrAccessDenied)
		return
	}

	if _, ok := c.GetQuery("notification"); ok {
		c.Header("Access-Control-Allow-Origin", "*")
		xmlConfig := models.Config{}
		data, _ := ioutil.ReadAll(c.Request.Body)
		xml.Unmarshal(data, &xmlConfig)
		xmlConfig.Bucket = bucket
		db := models.GetDB()

		if err := db.Create(&xmlConfig).Error; err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok {
				if mysqlErr.Number == 1062 {
					config := models.Config{}
					db.Where(&models.Config{Bucket: bucket}).
						Preload("Queues.Events").Preload("Queues.Resource").Preload("Queues.Filter.RuleList.Rules").
						Preload("Topics.Events").Preload("Topics.Resource").Preload("Topics.Filter.RuleList.Rules").
						First(&config)
					if len(xmlConfig.Queues) == 0 && len(xmlConfig.Topics) == 0 {
						db.Delete(&config)
						c.Status(http.StatusOK)
						return
					}
					for _, xmlQueue := range xmlConfig.Queues {
						targetResource, err := models.ParseARN(xmlQueue.ARN)
						if err != nil {
							writeErrorResponse(c, cmd.ErrARNNotification)
							return
						}
						if db.Where(models.Resource{
							AccountID: targetResource.AccountID,
							Service:   targetResource.Service,
							Name:      targetResource.Name,
						}).First(&targetResource).RecordNotFound() {
							writeErrorResponse(c, cmd.ErrARNNotification)
							return
						}

						queue := models.Queue{}
						if db.Where(models.Queue{
							QueueIdentifier: xmlQueue.QueueIdentifier,
							ConfigID:        config.ID,
						}).First(&queue).RecordNotFound() {
							xmlQueue.ResourceID = targetResource.ID
							xmlQueue.ConfigID = config.ID
							db.Create(&xmlQueue)
						} else {
							queue.ARN = targetResource.ARN()
							queue.ResourceID = targetResource.ID
							db.Save(&queue)
						}
					}

					for _, xmlTopic := range xmlConfig.Topics {
						targetResource, err := models.ParseARN(xmlTopic.ARN)
						if err != nil {
							writeErrorResponse(c, cmd.ErrARNNotification)
							return
						}
						if db.Where(models.Resource{
							AccountID: targetResource.AccountID,
							Service:   targetResource.Service,
							Name:      targetResource.Name,
						}).First(&targetResource).RecordNotFound() {
							writeErrorResponse(c, cmd.ErrARNNotification)
							return
						}

						topic := models.Topic{}
						if db.Where(models.Topic{
							TopicIdentifier: xmlTopic.TopicIdentifier,
							ConfigID:        config.ID,
						}).First(&topic).RecordNotFound() {
							xmlTopic.ResourceID = targetResource.ID
							xmlTopic.ConfigID = config.ID
							db.Create(&xmlTopic)
						} else {
							topic.ARN = targetResource.ARN()
							topic.ResourceID = targetResource.ID
							db.Save(&topic)
						}
					}
				}
			}
		} else {
			for _, queue := range xmlConfig.Queues {
				targetResource, err := models.ParseARN(queue.ARN)
				if err != nil {
					writeErrorResponse(c, cmd.ErrARNNotification)
					return
				}
				db.Where(models.Resource{
					AccountID: targetResource.AccountID,
					Service:   targetResource.Service,
					Name:      targetResource.Name,
				}).First(&targetResource)
				queue.ResourceID = targetResource.ID
				db.Save(&queue)
			}
			for _, topic := range xmlConfig.Topics {
				targetResource, err := models.ParseARN(topic.ARN)
				if err != nil {
					writeErrorResponse(c, cmd.ErrARNNotification)
					return
				}
				db.Where(models.Resource{
					AccountID: targetResource.AccountID,
					Service:   targetResource.Service,
					Name:      targetResource.Name,
				}).First(&targetResource)
				topic.ResourceID = targetResource.ID
				db.Save(&topic)
			}
		}

		c.Status(http.StatusOK)
		return
	}

	ReverseProxy()(c)
}

func checkResponse(resp *http.Response, method string, statusCode int) bool {
	clientReq := resp.Request

	if clientReq.Method == method && resp.StatusCode == statusCode {
		return true
	}

	return false
}

func getObjectName(req *http.Request) (bucketName string, objectName string, err error) {
	config := config.GetServerConfig()
	re := regexp.MustCompile("([A-Za-z0-9]*)\\." + config.Host)
	if group := re.FindStringSubmatch(req.Host); len(group) == 2 {
		bucketName = group[1]
		segments := strings.Split(req.URL.Path, "/")
		objectName = strings.Join(segments[1:], "/")
	} else { // path-style syntax
		segments := strings.Split(req.URL.Path, "/")
		bucketName = segments[1]
		objectName = strings.Join(segments[2:], "/")
	}

	return
}

func sendEvent(resp *http.Response, eventType event.Name) error {
	clientReq := resp.Request
	bucketName, objectName, _ := getObjectName(clientReq)

	client := models.GetCache()
	serverConfig := config.GetServerConfig()
	nConfig := models.Config{}
	db := models.GetDB()
	db.Where(&models.Config{Bucket: bucketName}).
		Preload("Queues.Events").Preload("Queues.Resource").Preload("Queues.Filter.RuleList.Rules").
		Preload("Topics.Events").Preload("Topics.Resource.Endpoints").Preload("Topics.Filter.RuleList.Rules").
		First(&nConfig)

	rulesMap := nConfig.ToRulesMap()
	eventTime := time.Now().UTC()

	var etag string
	if val, ok := resp.Header["Etag"]; ok {
		etag = val[0]
	}

	for _, resource := range rulesMap[eventType].Match(objectName) {
		newEvent := event.Event{
			EventVersion: "2.0",
			EventSource:  "aws:s3",
			AwsRegion:    serverConfig.Region,
			EventTime:    eventTime.Format("2006-01-02T15:04:05Z"),
			EventName:    eventType,
			UserIdentity: event.Identity{
				PrincipalID: "",
			},
			RequestParameters: map[string]string{
				"sourceIPAddress": clientReq.RemoteAddr,
			},
			ResponseElements: map[string]string{
				"x-amz-request-id": resp.Header["X-Amz-Request-Id"][0],
			},
			S3: event.Metadata{
				SchemaVersion:   "1.0",
				ConfigurationID: "Config",
				Bucket: event.Bucket{
					Name: bucketName,
					OwnerIdentity: event.Identity{
						PrincipalID: "",
					},
					ARN: resource.ARN(),
				},
				Object: event.Object{
					Key:       objectName,
					Size:      clientReq.ContentLength,
					ETag:      etag,
					Sequencer: fmt.Sprintf("%X", eventTime.UnixNano()),
				},
			},
		}

		value, err := json.Marshal(newEvent)
		if err != nil {
			panic(err)
		}

		switch resource.Service {
		case models.SQS:
			client.RPush(fmt.Sprintf("%s:%s:%s", resource.Service.String(), resource.AccountID, resource.Name), value)
		case models.SNS:
			celeryBroker, celeryBackend := models.GetCelery()
			celeryClient, _ := gocelery.NewCeleryClient(celeryBroker, celeryBackend, 0)

			for _, endpoint := range resource.Endpoints {
				celeryClient.Delay("worker.send_event", endpoint.URI, string(value))
			}
		}
	}

	return nil
}

func isMultipartUpload(request *http.Request) bool {
	q := request.URL.Query()
	return len(q["partNumber"]) != 0 && len(q["uploadId"]) != 0
}

func IsAdminUserPath(path string) bool {
	return path == "/admin/user/" || path == "/admin/user"
}

func ReverseProxy() gin.HandlerFunc {
	target := utils.GetEnv("TARGET_HOST", "127.0.0.1")

	return func(c *gin.Context) {
		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = target
		}

		modifyResponse := func(resp *http.Response) error {
			clientReq := resp.Request

			switch {
			case IsAdminUserPath(clientReq.URL.Path) && resp.StatusCode == 200:
				b, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				go HandleNfsExport(clientReq, b)
				resp.Body = ioutil.NopCloser(bytes.NewReader(b)) // put body back for client response
				return nil
			case len(clientReq.Header["X-Amz-Copy-Source"]) > 0:
				return sendEvent(resp, event.ObjectCreatedCopy)
			case checkResponse(resp, "POST", 200) && len(clientReq.URL.Query()["uploadId"]) != 0:
				go InheritNfsPermission(*clientReq)
				return sendEvent(resp, event.ObjectCreatedCompleteMultipartUpload)
			case len(resp.Header["Etag"]) > 0 && checkResponse(resp, "PUT", 200) && !isMultipartUpload(clientReq):
				go InheritNfsPermission(*clientReq)
				return sendEvent(resp, event.ObjectCreatedPut)
			case checkResponse(resp, "DELETE", 204):
				return sendEvent(resp, event.ObjectRemovedDelete)
			default:
				return nil
			}
		}

		proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: modifyResponse}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

type Grant struct {
	ID string `json:id`
}

type Policy struct {
	ACL ACL `json:"acl"`
}

type ACL struct {
	GrantMap []Grant `json:"grant_map"`
}

func getBucketUsers(bucketName string) (users []string, ok bool) {
	var policy Policy
	output, err := sh.Command("radosgw-admin", "policy", "--bucket="+bucketName).Output()
	if err != nil {
		return
	}

	err = json.Unmarshal(output, &policy)
	if err != nil {
		return
	}

	for _, grant := range policy.ACL.GrantMap {
		users = append(users, grant.ID)
	}

	return users, true
}

func contains(users []string, user string) bool {
	for _, u := range users {
		if u == user {
			return true
		}
	}

	return false
}
