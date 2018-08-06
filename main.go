package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/event"

	"gitlab.com/stor-inwinstack/kaoliang/pkg/controllers"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/models"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

var client *redis.Client
var globalServerConfig *ServerConfig
var targetList *event.TargetList

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	client = redis.NewClient(&redis.Options{
		Addr:     utils.GetEnv("REDIS_ADDR", "localhost:6789"),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	globalServerConfig = &ServerConfig{
		region: utils.GetEnv("RGW_REGION", "us-east-1"),
		host:   utils.GetEnv("", ""),
	}

	targetList = event.NewTargetList()

	models.SetDB()
	models.Migrate()
}

var errNoSuchNotifications = errors.New("The specified bucket does not have bucket notifications")

func main() {
	r := gin.Default()
	r.GET("/:bucket", func(c *gin.Context) {
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
		} else {
			reverseProxy()(c)
		}
	})

	r.PUT("/:bucket", func(c *gin.Context) {
		bucket := c.Param("bucket")

		_, notification := c.GetQuery("notification")

		if notification {
			region := globalServerConfig.getRegion()

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
		} else {
			reverseProxy()(c)
		}
	})

	r.GET("/", func(c *gin.Context) {
		action := c.Query("Action")
		switch action {
		case "CreateQueue":
			controllers.CreateQueue(c)
		default:
			reverseProxy()(c)
		}
	})

	r.NoRoute(reverseProxy())

	r.Run()
}

func writeErrorResponse(c *gin.Context, errorCode cmd.APIErrorCode) {
	apiError := cmd.GetAPIError(errorCode)
	errorResponse := cmd.GetAPIErrorResponse(apiError, c.Request.URL.Path)
	c.XML(apiError.HTTPStatusCode, errorResponse)
}

func readNotificationConfig(targetList *event.TargetList, bucket string) (*event.Config, error) {
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

	if err := client.Set(fmt.Sprintf("config:%s", bucket), output, 0).Err(); err != nil {
		return nil
	}

	return nil
}

type ServerConfig struct {
	region string
	host   string
}

func (config *ServerConfig) GetRegion() string {
	return config.region
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

func (config *ServerConfig) getRegion() string {
	return config.region
}

func sendEvent(resp *http.Response, eventType event.Name) error {
	clientReq := resp.Request
	bucketName, objectName, _ := getObjectName(clientReq)

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
			AwsRegion:    globalServerConfig.region,
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

func reverseProxy() gin.HandlerFunc {
	target := utils.GetEnv("TARGET_HOST", "127.0.0.1")

	return func(c *gin.Context) {
		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = target
		}

		modifyResponse := func(resp *http.Response) error {
			clientReq := resp.Request

			switch {
			case len(clientReq.Header["X-Amz-Copy-Source"]) > 0:
				return sendEvent(resp, event.ObjectCreatedCopy)
			case len(resp.Header["Etag"]) > 0 && checkResponse(resp, "PUT", 200):
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
