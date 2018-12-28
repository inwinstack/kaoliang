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
	Project      string    `json:"project"`
	ProjectId    string    `json:"project_id"`
	User         string    `json:"user"`
	Date         time.Time `json:"date"`
	Method       string    `json:"method"`
	Bucket       string    `json:"bucket"`
	Uri          string    `json:"uri"`
	ByteSend     int       `json:"byte_sned"`
	ByteRecieved int       `json:"byte_recieved"`
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

func LoggingOps(resp *http.Response) {
	// get date
	dateStr := resp.Header.Get("date")
	date, err := time.Parse(time.RFC1123, dateStr)
	if err != nil {
		date = time.Now()
	}
	// get response size
	byte_send := toInteger(resp.Header.Get("content-length"))
	// get received size
	byte_recieved := toInteger(resp.Request.Header.Get("content-length"))
	// get http method
	method := resp.Request.Method
	// get bucket and object
	bucket, _, _ := getObjectName(resp.Request)
	// get user id (project) and sub user
	accessKey := ExtractAccessKey(resp.Request)
	name, _, _ := cmd.GetCredentials(accessKey)

	index := strings.LastIndex(name, ":")
	var uid, subuser string
	if index == -1 {
		uid = name
		subuser = ""
	} else {
		uid = name[:index]
		subuser = name[index+1:]
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
	log := OperationLog{user.DisplayName, uid, subuser, date, method, bucket, resp.Request.RequestURI, byte_send, byte_recieved}
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
