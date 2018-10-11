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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/event"
	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/models"
	"github.com/inwinstack/kaoliang/pkg/utils"
)

var targetList *event.TargetList
var errNoSuchNotifications = errors.New("The specified bucket does not have bucket notifications")

func GetBucketNotification(c *gin.Context) {
	_, err := authenticate(c.Request)
	if err != cmd.ErrNone {
		writeErrorResponse(c, err)
	}

	bucket := c.Param("bucket")

	_, notification := c.GetQuery("notification")

	if notification {
		nConfig, err := readNotificationConfig(targetList, bucket)
		if err != nil {
			if err != errNoSuchNotifications {
				writeErrorResponse(c, cmd.ToAPIErrorCode(err))
				return
			}

			nConfig = &event.Config{}
		}

		c.XML(http.StatusOK, nConfig)
		return
	}

	ReverseProxy()(c)
}

func PutBucketNotification(c *gin.Context) {
	_, err := authenticate(c.Request)
	if err != cmd.ErrNone {
		writeErrorResponse(c, err)
	}

	bucket := c.Param("bucket")
	serverConfig := config.GetServerConfig()

	_, notification := c.GetQuery("notification")

	if notification {
		region := serverConfig.Region

		config, err := event.ParseConfig(c.Request.Body, region, targetList)
		if err != nil {
			apiErr := cmd.ErrMalformedXML
			if event.IsEventError(err) {
				apiErr = cmd.ToAPIErrorCode(err)
			}

			writeErrorResponse(c, apiErr)
			return
		}

		if err = saveNotificationConfig(config, bucket); err != nil {
			writeErrorResponse(c, cmd.ToAPIErrorCode(err))
			return
		}

		c.Status(http.StatusOK)
		return
	}

	ReverseProxy()(c)
}

func readNotificationConfig(targetList *event.TargetList, bucket string) (*event.Config, error) {
	client := models.GetCache()
	val, err := client.Get(fmt.Sprintf("config:%s", bucket)).Result()
	if err != nil {
		return nil, errNoSuchNotifications
	}

	config, err := event.ParseConfig(strings.NewReader(val), "us-east-1", targetList)

	return config, err
}

func saveNotificationConfig(conf *event.Config, bucket string) error {
	output, err := xml.Marshal(conf)
	if err != nil {
		return nil
	}

	client := models.GetCache()
	if err := client.Set(fmt.Sprintf("config:%s", bucket), output, 0).Err(); err != nil {
		return nil
	}

	return nil
}

func checkResponse(resp *http.Response, method string, statusCode int) bool {
	clientReq := resp.Request

	if clientReq.Method == method && resp.StatusCode == statusCode {
		return true
	}

	return false
}

// currently only supports path-style syntax
func getObjectName(req *http.Request) (string, string, error) {
	segments := strings.Split(req.URL.Path, "/")
	bucketName := segments[1]
	objectName := segments[2]

	return bucketName, objectName, nil
}

func sendEvent(resp *http.Response, eventType event.Name) error {
	clientReq := resp.Request
	bucketName, objectName, _ := getObjectName(clientReq)

	client := models.GetCache()
	serverConfig := config.GetServerConfig()
	nConfig, err := readNotificationConfig(targetList, bucketName)
	if err != nil {
		panic(err)
	}

	rulesMap := nConfig.ToRulesMap()
	eventTime := time.Now().UTC()

	var etag string
	if val, ok := resp.Header["Etag"]; ok {
		etag = val[0]
	}

	for targetID := range rulesMap[eventType].Match(objectName) {
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
					ARN: "",
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

		client.RPush(fmt.Sprintf("%s:%s:%s", targetID.Service, targetID.ID, targetID.Name), value)
	}

	return err
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
			case len(resp.Header["Etag"]) > 0 && checkResponse(resp, "PUT", 200):
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
