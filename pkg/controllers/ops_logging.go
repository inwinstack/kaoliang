package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	sh "github.com/codeskyblue/go-sh"
	"github.com/minio/minio/cmd"
)

type OperationLog struct {
	Project      string `json:"project"`
	User         string `json:"user"`
	Date         string `json:"date"`
	Method       string `json:"method"`
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

func LoggingOps(resp *http.Response) {
	// get date
	date := resp.Header.Get("date")
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

	log := OperationLog{user.DisplayName, subuser, date, method, bucket, resp.Request.RequestURI, byte_send, byte_recieved}
	data, err := json.Marshal(log)
	fmt.Println(string(data))
}
