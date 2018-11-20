package controllers

import (
	"bytes"
	"io/ioutil"

	"github.com/gin-gonic/gin"
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
