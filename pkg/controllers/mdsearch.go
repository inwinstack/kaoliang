package controllers

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/olivere/elastic"
	uuid "github.com/satori/go.uuid"

	"github.com/inwinstack/kaoliang/pkg/models"
	"github.com/inwinstack/kaoliang/pkg/utils"
)

type Object struct {
	Bucket         string    `json:"Bucket"`
	Key            string    `json:"Key"`
	Instance       string    `json:"Instance"`
	VersionedEpoch int64     `json:"VersionedEpoch"`
	LastModified   time.Time `json:"LastModified"`
	Size           int64     `json:"Size"`
	Etag           string    `json:"ETag"`
	ContentType    string    `json:"ContentType"`
	Owner          struct {
		ID          string `json:"ID"`
		DisplayName string `json:"DisplayName"`
	} `json:"Owner"`
	CustomMetadata []CustomMetadataEntry `json:"CustomMetadata"`
}

type CustomMetadataEntry struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type SearchResponse struct {
	Marker      string
	IsTruncated string
	Objects     []Object
}

type ObjectType struct {
	Bucket   string `json:"bucket"`
	Instance string `json:"instance"`
	Name     string `json:"name"`
	Owner    struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	} `json:"owner"`
	Meta struct {
		ContentType           string         `json:"content_type"`
		Etag                  string         `json:"etag"`
		Mtime                 time.Time      `json:"mtime"`
		Size                  int64          `json:"size"`
		TailTag               string         `json:"tail_tag"`
		XAmzAcl               string         `json:"x-amz-acl"`
		XAmzContentSha256     string         `json:"x-amz-content-sha256"`
		XAmzCopySource        string         `json:"x-amz-copy-source"`
		XAmzDate              string         `json:"x-amz-date"`
		XAmzMetadataDirective string         `json:"x-amz-metadata-directive"`
		XAmzStorageClass      string         `json:"x-amz-storage-class"`
		CustomString          []CustomString `json:"custom-string"`
	} `json:"meta"`
	Permissions    []string `json:"permissions"`
	VersionedEpoch int64    `json:"versioned_epoch"`
}

type CustomString struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func escape(s string) (escaped string) {
	escaped = strings.Replace(s, "\n", "<br>", -1)
	escaped = strings.Replace(escaped, "\t", " ", -1)
	return
}

func makeInvalidSyntaxResponse(requestID string) ErrorResponse {

	return ErrorResponse{
		Type: "Sender",
		Code: "InvalidSyntax",
		Message: escape("Syntax should be one of following: name==(filename), contenttype==(type), " +
			"lastmodified(< or <= or > or >=)(duration or YYYY-MM-DDThh:mm)," +
			"size(<= or < or >= or >)(bytes), etag==(MD5 hash value)"),
		RequestID: requestID,
	}

}

func Search(c *gin.Context) {
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

	requestID, _ := uuid.NewV4()
	query := c.Query("query")

	if query == "" {
		body := makeInvalidSyntaxResponse(requestID.String())
		c.JSON(http.StatusBadRequest, body)
		return
	}

	index := utils.GetEnv("METADATA_INDEX", "")
	from, err := strconv.Atoi(c.Query("marker"))
	if err != nil {
		from = 0
	}
	size, err := strconv.Atoi(c.Query("max-keys"))
	if err != nil {
		size = 100
	}

	ctx := context.Background()
	client := models.GetElasticsearch()
	if client == nil {
		c.Status(http.StatusGatewayTimeout)
		return
	}

	boolQuery := elastic.NewBoolQuery()
	boolQuery = boolQuery.Filter(elastic.NewTermQuery("bucket", bucket))

	re := regexp.MustCompile("^(name|lastmodified|contenttype|size|etag|x-amz-meta-.+)\\s*(<=|<|==|>=|>)\\s*([^\\s]+)$")
	group := re.FindStringSubmatch(strings.TrimSpace(query))
	if len(group) != 4 {
		body := makeInvalidSyntaxResponse(requestID.String())
		c.JSON(http.StatusBadRequest, body)
		return
	}

	switch {
	case group[1] == "name":
		if group[2] != "==" {
			body := ErrorResponse{
				Type:      "Sender",
				Code:      "InvalidSyntax",
				Message:   "Syntax should be name==(filename), the filename is a string and support wildcard character e.g. user_*",
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
		if strings.Contains(group[3], "*") {
			boolQuery = boolQuery.Must(elastic.NewWildcardQuery("name", group[3]))
		} else {
			boolQuery = boolQuery.Must(elastic.NewTermQuery("name", group[3]))
		}
	case group[1] == "contenttype":
		if group[2] != "==" {
			body := ErrorResponse{
				Type:      "Sender",
				Code:      "InvalidSyntax",
				Message:   "Syntax should be contenttype==(type), the type is a string and support wildcard character e.g. image/*",
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
		if strings.Contains(group[3], "*") {
			boolQuery = boolQuery.Must(elastic.NewWildcardQuery("meta.content_type", group[3]))
		} else {
			boolQuery = boolQuery.Must(elastic.NewTermQuery("meta.content_type", group[3]))
		}
	case group[1] == "lastmodified":
		duration := regexp.MustCompile("^[1-9][0-9]*[s|m|h|d|w|M|y]$")
		matchedDuration := duration.MatchString(group[3])
		if matchedDuration {
			switch group[2] {
			case "<=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Gte(fmt.Sprintf("now-%s", group[3])).Lte("now"))
			case "<":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Gt(fmt.Sprintf("now-%s", group[3])).Lt("now"))
			case ">=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Lte(fmt.Sprintf("now-%s", group[3])))
			case ">":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Lt(fmt.Sprintf("now-%s", group[3])))
			default:
				body := ErrorResponse{
					Type: "Sender",
					Code: "InvalidSyntax",
					Message: escape("Syntax should be lastmodified<=(duration), lastmodified<(duration)," +
						"lastmodified>=(duration) or lastmodified>(duration)\n\n." +
						"Duration can accept seconds, minutes, hours, days, weeks, months and years. e.g. 30s, 5m, 6h, 1d, 7w, 3M, 2y."),
					RequestID: requestID.String(),
				}
				c.JSON(http.StatusBadRequest, body)
				return
			}
		}
		startTime, err := time.Parse("2006-01-02T15:04", group[3])
		if err == nil {
			startTimeISO := startTime.Format("2006-01-02T15:04")
			switch group[2] {
			case "<=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Lte(fmt.Sprintf("%s", startTimeISO)))
			case "<":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Lt(fmt.Sprintf("%s", startTimeISO)))
			case ">=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Gte(fmt.Sprintf("%s", startTimeISO)))
			case ">":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.mtime").Gt(fmt.Sprintf("%s", startTimeISO)))
			default:
				body := ErrorResponse{
					Type: "Sender",
					Code: "InvalidSyntax",
					Message: "Syntax should be lastmodified<=(YYYY-MM-DDThh:mm), lastmodified<(YYYY-MM-DDThh:mm)," +
						"lastmodified>=(YYYY-MM-DDThh:mm) or lastmodified<=(YYYY-MM-DDThh:mm) e.g. 2018-05-26T03:48",
					RequestID: requestID.String(),
				}
				c.JSON(http.StatusBadRequest, body)
				return
			}
		}

		if !matchedDuration && (startTime == time.Time{}) {
			body := ErrorResponse{
				Type: "Sender",
				Code: "InvalidSyntax",
				Message: escape("Syntanx should be lastmodified<=(duration or YYYY-MM-DDThh:mm), lastmodified<=(duration or YYYY-MM-DDThh:mm)," +
					"lastmodified<=(duration or YYYY-MM-DDThh:mm) or lastmodified<=(duration or YYYY-MM-DDThh:mm).\n\n" +
					"Durations can accept seconds, minutes, hours, days, weeks, months and years. e.g. 30s, 5m, 6h, 1d, 7w, 3m, 2y."),
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
	case group[1] == "size":
		size, err := strconv.Atoi(group[3])
		if err == nil && size >= 0 {
			switch group[2] {
			case "<=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.size").Lte(fmt.Sprintf("%d", size)))
			case "<":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.size").Lt(fmt.Sprintf("%d", size)))
			case ">=":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.size").Gte(fmt.Sprintf("%d", size)))
			case ">":
				boolQuery = boolQuery.Filter(elastic.NewRangeQuery("meta.size").Gt(fmt.Sprintf("%d", size)))
			default:
				body := ErrorResponse{
					Type: "Sender",
					Code: "InvalidSyntax",
					Message: "Syntax should be size<=(bytes), size<(bytes), size>=(bytes) or size>(bytes) " +
						"and the bytes must be integer and greater than or equal to 0.",
					RequestID: requestID.String(),
				}
				c.JSON(http.StatusBadRequest, body)
				return
			}
		} else {
			body := ErrorResponse{
				Type: "Sender",
				Code: "InvalidSyntax",
				Message: "Syntax should be size<=(bytes), size<(bytes), size>=(bytes) or size>(bytes) " +
					"and the bytes must be integer and greater than or equal to 0.",
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
	case group[1] == "etag":
		etag := regexp.MustCompile("^[a-f0-9]{32}$")
		if group[2] == "==" && etag.MatchString(group[3]) {
			boolQuery = boolQuery.Must(elastic.NewTermQuery("meta.etag", group[3]))
		} else {
			body := ErrorResponse{
				Type:      "Sender",
				Code:      "InvalidSyntax",
				Message:   "Syntax should be etag==(MD5 hash value)",
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}
	case strings.Contains(group[1], "x-amz-meta-"):
		if group[2] != "==" {
			body := ErrorResponse{
				Type: "Sender",
				Code: "InvalidSyntax",
				Message: "Syntax should be x-amx-meta-(name)==(value), " +
					"the name should be a string and the value is a string which support wildcard character " +
					"e.g. x-amz-meta-serialnumber==a9507*",
				RequestID: requestID.String(),
			}
			c.JSON(http.StatusBadRequest, body)
			return
		}

		// take custom metadata name from query parameter
		customMetaName := strings.Replace(group[1], "x-amz-meta-", "", 1)

		// add nested query for metadata
		bq := elastic.NewBoolQuery()
		bq = bq.Must(elastic.NewTermQuery("meta.custom-string.name", customMetaName))
		if strings.Contains(group[3], "*") {
			bq = bq.Must(elastic.NewWildcardQuery("meta.custom-string.value", group[3]))
		} else {
			bq = bq.Must(elastic.NewTermQuery("meta.custom-string.value", group[3]))
		}
		q := elastic.NewNestedQuery("meta.custom-string", bq)
		boolQuery = boolQuery.Must(q)
	default:
		body := makeInvalidSyntaxResponse(requestID.String())
		c.JSON(http.StatusBadRequest, body)
		return
	}

	searchResult, err := client.Search().
		Index(index).
		Query(boolQuery).
		From(from).
		Size(size).
		Pretty(true).
		Do(ctx)

	if err != nil {
		panic(err)
	}

	searchResp := SearchResponse{
		IsTruncated: "false",
	}

	var objs []Object
	for _, document := range searchResult.Each(reflect.TypeOf(ObjectType{})) {
		if d, ok := document.(ObjectType); ok {
			obj := Object{
				Bucket:         d.Bucket,
				Key:            d.Name,
				Instance:       d.Instance,
				VersionedEpoch: d.VersionedEpoch,
				LastModified:   d.Meta.Mtime,
				Size:           d.Meta.Size,
				Etag:           fmt.Sprintf("\\\"%s\"\\", d.Meta.Etag),
				ContentType:    d.Meta.ContentType,
				Owner: struct {
					ID          string `json:"ID"`
					DisplayName string `json:"DisplayName"`
				}{
					d.Owner.ID,
					d.Owner.DisplayName,
				},
				CustomMetadata: []CustomMetadataEntry{},
			}
			for _, cs := range d.Meta.CustomString {
				cme := CustomMetadataEntry{Name: cs.Name, Value: cs.Value}
				obj.CustomMetadata = append(obj.CustomMetadata, cme)
			}
			objs = append(objs, obj)
		}
	}

	searchResp.Objects = objs
	c.JSON(http.StatusOK, searchResp)
}
