package controllers

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio/cmd"
)

func PostForm(c *gin.Context, key string) string {
	buf, _ := ioutil.ReadAll(c.Request.Body)
	bufForPostForm := ioutil.NopCloser(bytes.NewBuffer(buf))
	duplicateBuf := ioutil.NopCloser(bytes.NewBuffer(buf))
	c.Request.Body = bufForPostForm
	value := c.PostForm(key)
	c.Request.Body = duplicateBuf

	return value
}

func ExtractAccessKeyV4(auth string) string {
	tokens := strings.Split(auth, " ")
	if len(tokens) != 2 {
		return ""
	}
	credential := strings.Split(tokens[1], ",")[0]
	creds := strings.Split(strings.TrimSpace(credential), "=")
	if len(creds) != 2 {
		return ""
	}
	if creds[0] != "Credential" {
		return ""
	}
	credElements := strings.Split(strings.TrimSpace(creds[1]), "/")
	if len(credElements) < 5 {
		return ""
	}
	accessKey := strings.Join(credElements[:len(credElements)-4], "/")
	return accessKey
}

func ExtractAccessKeyV2(auth string) string {
	tokens := strings.Split(auth, " ")
	if len(tokens) != 2 {
		return ""
	}
	return strings.Split(tokens[1], ":")[0]
}

func ExtractAccessKey(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	authType := cmd.GetRequestAuthType(req)
	if authType == 7 { // authTypeSignedV2
		return ExtractAccessKeyV2(auth)
	} else if authType == 6 { // authTypeSigned
		return ExtractAccessKeyV4(auth)
	} else {
		return ""
	}
}
