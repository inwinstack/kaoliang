package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ceph/go-ceph/rados"
	sh "github.com/codeskyblue/go-sh"
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/minio/minio/cmd"
)

type OperationLog struct {
	Project      string `json:"project"`
	ProjectId    string `json:"project_id"`
	User         string `json:"user"`
	Date         string `json:"date"`
	Method       string `json:"method"`
	StatusCode   string `json:"status_code"`
	Bucket       string `json:"bucket"`
	Uri          string `json:"uri"`
	ByteSend     int    `json:"byte_sned"`
	ByteRecieved int    `json:"byte_recieved"`
}

func toInteger(contentLength string) int {
	if contentLength == "" {
		return 0
	}
	i, err := strconv.Atoi(contentLength)
	if err != nil {
		return 0
	}
	return i
}

func isExists(bucket string) bool {
	if bucket == "" {
		return false
	}
	output, err := sh.Command("radosgw-admin", "bucket", "list").Output()
	if err != nil {
		fmt.Println("Can not get bucket list", bucket)
		return false
	}
	var buckets []string
	err = json.Unmarshal([]byte(output), &buckets)
	if err != nil {
		fmt.Println("Can not parse bucket list", bucket)
		return false
	}
	for _, b := range buckets {
		if b == bucket {
			return true
		}
	}
	return false
}

func extractUserInfo(req *http.Request) (string, string) {
	accessKey := ExtractAccessKey(req)
	name, _, _ := cmd.GetCredentials(accessKey)
	if name == "" {
		return "", ""
	}

	index := strings.LastIndex(name, ":")
	if index == -1 {
		return name, ""
	} else {
		return name[:index], name[index+1:]
	}
}

func LoggingOps(resp *http.Response) {
	// get bucket
	bucket, _, _ := getObjectName(resp.Request)
	if bucket == "" || bucket == "admin" {
		// only record bucket operation
		return
	}
	// get date
	dateStr := resp.Header.Get("date")
	date, err := time.Parse(time.RFC1123, dateStr)
	if err != nil {
		date = time.Now()
	}
	// get response size
	byteSend := toInteger(resp.Header.Get("content-length"))
	// get received size
	byteRecieved := toInteger(resp.Request.Header.Get("content-length"))
	// get http method
	method := resp.Request.Method
	// get http status
	statusCode := strconv.Itoa(resp.StatusCode)
	// get user id (project) and sub user
	uid, subuser := extractUserInfo(resp.Request)
	if uid == "" {
		// can not found user by access key
		return
	}
	// get display name
	output, err := sh.Command("radosgw-admin", "user", "info", "--uid", uid).Output()
	if err != nil {
		fmt.Println("Can not found the info of uid", uid)
		return
	}
	var user RgwUser
	err = json.Unmarshal(output, &user)
	if err != nil {
		fmt.Println("Can not parse user info", uid)
		return
	}
	// generate ops log json object
	log := OperationLog{user.DisplayName, uid, subuser, date.Format(time.RFC3339), method, statusCode, bucket, resp.Request.RequestURI, byteSend, byteRecieved}
	data, err := json.Marshal(log)
	if err != nil {
		fmt.Println("operation log can not be generated", uid)
		return
	}
	data = append(data, "\n"...)

	logObjName := "ops_" + bucket + "_" + date.Format("2006-01-02-15") + ".log"

	poolName := utils.GetEnv("RGW_OPS_LOG_POOL", "us-east-1.rgw.opslog")

	// write data
	conn, _ := rados.NewConnWithUser("admin")
	conn.ReadDefaultConfigFile()
	conn.Connect()
	defer conn.Shutdown()
	ioctx, _ := conn.OpenIOContext(poolName)
	defer ioctx.Destroy()

	ioctx.Append(logObjName, data)
}
