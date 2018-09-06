package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/ceph/go-ceph/rados"
	"gitlab.com/stor-inwinstack/kaoliang/pkg/utils"
)

type RgwUser struct {
	UserId string   `json:"user_id"`
	Keys   []RgwKey `json:"keys"`
}

type RgwKey struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func random(min int, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

func connect() (*rados.Conn, *rados.IOContext) {
	nfsCfgUser := utils.GetEnv("NFS_CONFIG_User", "admin")
	nfsCfgPool := utils.GetEnv("NFS_CONFIG_POOL", "nfs-ganesha")

	// connect rados
	conn, _ := rados.NewConnWithUser(nfsCfgUser)
	conn.ReadDefaultConfigFile()
	conn.Connect()
	ioctx, _ := conn.OpenIOContext(nfsCfgPool)
	return conn, ioctx
}

func addNfsExport(body []byte) {
	// get user info
	var userData RgwUser
	err := json.Unmarshal(body, &userData)
	if err != nil {
		return
	}
	// only export when create user (same request only add key on second times)
	if len(userData.Keys) > 1 {
		return
	}
	nfsCfgPool := utils.GetEnv("NFS_CONFIG_POOL", "nfs-ganesha")
	nfsCfgName := utils.GetEnv("NFS_CONFIG_NAME", "export")

	conn, ioctx := connect()
	defer ioctx.Destroy()
	defer conn.Shutdown()

	// create export obj
	exportObjName := createNfsExportObj(ioctx, &userData)
	// add export obj path to export list
	addExportPathToList(ioctx, nfsCfgName, nfsCfgPool, exportObjName)
}

func removeNfsExport(userId string) {
	nfsCfgPool := utils.GetEnv("NFS_CONFIG_POOL", "nfs-ganesha")
	nfsCfgName := utils.GetEnv("NFS_CONFIG_NAME", "export")

	conn, ioctx := connect()
	defer ioctx.Destroy()
	defer conn.Shutdown()

	exportObjName := makeExportObjName(userId)
	// remove export obj path to export list
	removeExportPathToList(ioctx, nfsCfgName, nfsCfgPool, exportObjName)
	// remove export obj
	removeNfsExportObj(ioctx, exportObjName)
}

func makeExportObjName(userId string) string {
	return fmt.Sprintf("export_%s", userId)
}

func makeExport(poolName, exportObjName string) string {
	return fmt.Sprintf("%%url \"rados://%s/%s\"\n", poolName, exportObjName)
}

func addExportPathToList(ioctx *rados.IOContext, exportName string, poolName string, exportObjName string) {
	lock := "export_add_lock"
	cookie := "export_add_cookie"
	newExport := makeExport(poolName, exportObjName)
	ioctx.LockExclusive(exportName, lock, cookie, "add export", 0, nil)
	ioctx.Append(exportName, []byte(newExport))
	ioctx.Unlock(exportName, lock, cookie)
}

func removeExportPathToList(ioctx *rados.IOContext, exportName string, poolName string, exportObjName string) {
	lock := "export_remove_lock"
	cookie := "export_remove_cookie"

	targetExport := makeExport(poolName, exportObjName)
	ioctx.LockExclusive(exportName, lock, cookie, "export_append", 0, nil)
	// read all export list
	stat, _ := ioctx.Stat(exportName)
	size := stat.Size
	data := make([]byte, size)
	ioctx.Read(exportName, data, 0)
	// remove target export and write back
	removedData := strings.Replace(string(data), targetExport, "", 1)
	ioctx.WriteFull(exportName, []byte(removedData))
	ioctx.Unlock(exportName, lock, cookie)
}

func createNfsExportObj(ioctx *rados.IOContext, data *RgwUser) string {
	userId := data.UserId
	accessKey := data.Keys[0].AccessKey
	secretKey := data.Keys[0].SecretKey

	exportId := random(1, 65535)
	exportTemp := `Export {
	Export_ID = %d;
	Path = "/";
	Pseudo = "/%s";
	Access_Type = RW;
	Protocols = 4;
	Transports = TCP;
	FSAL {
		Name = RGW; 
		User_Id = "%s"; 
		Access_Key_Id ="%s";
                Secret_Access_Key = "%s";
        }
}`
	exportObjName := makeExportObjName(userId)
	export := fmt.Sprintf(exportTemp, exportId, userId, userId, accessKey, secretKey)
	ioctx.WriteFull(exportObjName, []byte(export))
	return exportObjName
}

func removeNfsExportObj(ioctx *rados.IOContext, exportObjName string) {
	ioctx.Delete(exportObjName)
}

func HandleNfsExport(req *http.Request, body []byte) {
	_, isSubuser := req.URL.Query()["subuser"]
	_, isKey := req.URL.Query()["key"]
	_, isQuota := req.URL.Query()["quota"]
	_, isCaps := req.URL.Query()["caps"]

	// only handle user related request
	if isSubuser || isKey || isQuota || isCaps {
		return
	}
	// handle create user
	if req.Method == "PUT" {
		addNfsExport(body)
		return
	}
	if req.Method == "DELETE" {
		uid, _ := req.URL.Query()["uid"]
		removeNfsExport(uid[0])
		return
	}
}
