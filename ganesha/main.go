package main

import (
	"github.com/ceph/go-ceph/rados"
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
	"strconv"
)

type Ganesha struct {
	Pool          string
	ExportsConfig string
}

type ExportSummary struct {
	Uid      string `json:"uid"`
	Pseudo   string `json:"pseudo"`
	ExportId int    `json:"export_id"`
}

func (g Ganesha) GetExports(c *gin.Context) {
	// connect to rados
	conn, err := rados.NewConn()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"status": http.StatusInternalServerError, "message": "could not initailize backend connection"})
		return
	}
	err = conn.ReadDefaultConfigFile()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"status": http.StatusInternalServerError, "message": "could not initailize backend connection"})
		return
	}
	err = conn.Connect()
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"status": http.StatusInternalServerError, "message": "could not connect to backend server"})
		return
	}
	defer conn.Shutdown()

	ioctx, err := conn.OpenIOContext(g.Pool)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"status": http.StatusInternalServerError, "message": "could not connect to backend server"})
		return
	}
	defer ioctx.Destroy()

	// load ganesha export config list
	stat, err := ioctx.Stat(g.ExportsConfig)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"status": http.StatusInternalServerError, "message": "could not load export list"})
		return
	}
	size := stat.Size
	data := make([]byte, size)
	ioctx.Read(g.ExportsConfig, data, 0)

	// parser to export uid list
	exportSummarys := make([]ExportSummary, 0)
	pattern := regexp.MustCompile("%url \"rados://nfs-ganesha/export_([0-9a-zA-Z-]+)\"")
	match := pattern.FindAllStringSubmatch(string(data), -1)

	// get pseudo (export path) and export id for output
	tmp := make([]byte, 100)
	for i := range match {
		uid := match[i][1]

		len, err := ioctx.GetXattr("export_"+uid, "export_id", tmp)
		if err != nil {
			continue
		}
		exportId, err := strconv.Atoi(string(tmp[0:len]))
		if err != nil {
			continue
		}
		len, err = ioctx.GetXattr("export_"+uid, "pseudo", tmp)
		if err != nil {
			continue
		}
		pseudo := string(tmp[0:len])
		es := ExportSummary{Uid: uid, ExportId: exportId, Pseudo: pseudo}
		exportSummarys = append(exportSummarys, es)
	}
	c.JSON(
		http.StatusOK,
		gin.H{"status": http.StatusOK, "message": exportSummarys})
}

func main() {
	r := gin.Default()
	r.RedirectTrailingSlash = false

	g := Ganesha{Pool: "nfs-ganesha", ExportsConfig: "export"}

	r.GET("/admin/ganesha", g.GetExports)

	r.Run()
}
