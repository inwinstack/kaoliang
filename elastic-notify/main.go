package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gocelery/gocelery"
	"github.com/gorilla/websocket"
	"github.com/inwinstack/kaoliang/pkg/config"
	"github.com/inwinstack/kaoliang/pkg/models"
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/joho/godotenv"

	"github.com/minio/minio/pkg/event"
)

type Owner struct {
	DisplayName string `json:"display_name"`
	ID          string `json:"id"`
}

type Metadata struct {
	Etag string `json:"etag"`
	Size int64  `json:"size"`
}

type Source struct {
	Bucket   string   `json:"bucket"`
	Object   string   `json:"name"`
	Metadata Metadata `json:"meta"`
	Owner    Owner    `json:"owner"`
}

type Change struct {
	Version   int    `json:"_version"`
	Operation string `json:"_operation"`
	Source    Source `json:"_source"`
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	config.SetServerConfig()
	models.SetDB()
	models.Migrate()
	models.SetCache()
	models.SetCelery()
}

func main() {
	for {
		addrs := strings.Split(utils.GetEnv("CHANGES_ADDR", "localhost:9400"), ", ")

		for _, addr := range addrs {
			changesAddr := flag.String("addr", strings.Trim(addr, " "), "http service address")
			flag.Parse()

			u := url.URL{
				Scheme: "ws",
				Host:   *changesAddr,
				Path:   "/ws/_changes",
			}

			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				log.Printf("An error occurred while dialing to feed of changes service. %s\n", err)
				continue
			}

			log.Printf("Connected to %s\n", strings.Trim(addr, " "))

			defer c.Close()

			cfg := config.GetServerConfig()

			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Printf("An error occurred while reading message from WebSocket connection. %s\n", err)
					break
				}

				change := Change{}
				json.Unmarshal(message, &change)

				if change.Version >= 1 {
					log.Printf("Operation: %s, Bucket: %s, Object: %s", change.Operation, change.Source.Bucket, change.Source.Object)
					switch {
					case (change.Operation == "CREATE" || change.Operation == "INDEX") && cfg.EnableElasticCreate == "True":
						sendEvent(change, event.ObjectCreatedPut)
					}
				}
			}
		}
	}
}

func sendEvent(change Change, eventType event.Name) error {
	bucketName := change.Source.Bucket
	objectName := change.Source.Object
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
				"sourceIPAddress": "",
			},
			ResponseElements: map[string]string{
				"x-amz-request-id": "",
			},
			S3: event.Metadata{
				SchemaVersion:   "1.0",
				ConfigurationID: "Config",
				Bucket: event.Bucket{
					Name: bucketName,
					OwnerIdentity: event.Identity{
						PrincipalID: change.Source.Owner.DisplayName,
					},
					ARN: resource.ARN(),
				},
				Object: event.Object{
					Key:       objectName,
					Size:      change.Source.Metadata.Size,
					ETag:      change.Source.Metadata.Etag,
					Sequencer: fmt.Sprintf("%X", eventTime.UnixNano()),
				},
			},
		}

		value, _ := json.Marshal(newEvent)

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
