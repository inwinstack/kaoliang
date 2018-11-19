package controllers

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
	"github.com/olivere/elastic"

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
		ContentType           string    `json:"content_type"`
		Etag                  string    `json:"etag"`
		Mtime                 time.Time `json:"mtime"`
		Size                  int64     `json:"size"`
		TailTag               string    `json:"tail_tag"`
		XAmzAcl               string    `json:"x-amz-acl"`
		XAmzContentSha256     string    `json:"x-amz-content-sha256"`
		XAmzCopySource        string    `json:"x-amz-copy-source"`
		XAmzDate              string    `json:"x-amz-date"`
		XAmzMetadataDirective string    `json:"x-amz-metadata-directive"`
		XAmzStorageClass      string    `json:"x-amz-storage-class"`
	} `json:"meta"`
	Permissions    []string `json:"permissions"`
	VersionedEpoch int64    `json:"versioned_epoch"`
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

	if query := c.Query("query"); query != "" {
		index := utils.GetEnv("METADATA_INDEX", "")
		bucket := c.Param("bucket")
		from, err := strconv.Atoi(c.Query("marker"))
		if err != nil {
			from = 0
		}
		size, err := strconv.Atoi(c.Query("max-keys"))
		if err != nil {
			size = 100
		}

		ctx := context.Background()
		client, err := elastic.NewClient(
			elastic.SetURL(utils.GetEnv("ELS_URL", "http://localhost:9200")),
		)
		if err != nil {
			c.Status(http.StatusGatewayTimeout)
		}

		boolQuery := elastic.NewBoolQuery()

		re := regexp.MustCompile("(name|lastmodified|contenttype|size|etag)(<=|<|==|=|>=|>)(.+)")
		if group := re.FindStringSubmatch(query); len(group) == 4 {
			switch group[1] {
			case "name":
				if group[2] != "==" {
					c.Status(http.StatusBadRequest)
					return
				}
				boolQuery = boolQuery.Must(elastic.NewWildcardQuery("name", group[3]))
				boolQuery = boolQuery.Filter(elastic.NewTermQuery("bucket", bucket))
			case "content_type":
				if group[2] != "==" {
					c.Status(http.StatusBadRequest)
					return
				}
				boolQuery = boolQuery.Must(elastic.NewWildcardQuery("meta.content_type", group[3]))
				boolQuery = boolQuery.Filter(elastic.NewTermQuery("bucket", bucket))
			case "last_modified":
				boolQuery = boolQuery.Must(elastic.NewMatchQuery("bucket", bucket))
				duration, err := time.ParseDuration(group[3])
				if err == nil {
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
						c.Status(http.StatusBadRequest)
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
						c.Status(http.StatusBadRequest)
						return
					}
				}

				if duration == time.Duration(0) && (startTime == time.Time{}) {
					c.Status(http.StatusBadRequest)
					return
				}
			case "size":
				size, err := strconv.Atoi(group[3])
				if err == nil && size >= 0 {
					boolQuery = boolQuery.Must(elastic.NewMatchQuery("bucket", bucket))
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
						c.Status(http.StatusBadRequest)
						return
					}
				} else {
					c.Status(http.StatusBadRequest)
					return
				}
			case "etag":
				etag := regexp.MustCompile("^[a-f0-9]{32}$")
				if group[2] == "==" && etag.MatchString(group[3]) {
					boolQuery = boolQuery.Must(elastic.NewTermQuery("meta.etag", group[3]))
					boolQuery = boolQuery.Filter(elastic.NewTermQuery("bucket", bucket))
				} else {
					c.Status(http.StatusBadRequest)
					return
				}
			}
		} else {
			c.Status(http.StatusBadRequest)
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
				}
				objs = append(objs, obj)
			}
		}

		searchResp.Objects = objs
		c.JSON(http.StatusOK, searchResp)
	}
}
