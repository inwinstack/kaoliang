/*
Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rgw"
	sh "github.com/codeskyblue/go-sh"
	"github.com/gin-gonic/gin"
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/minio/minio/cmd"
)

type RgwUser struct {
	UserId      string   `json:"user_id"`
	DisplayName string   `json:"display_name"`
	MaxBuckets  int      `json:"max_buckets"`
	Keys        []RgwKey `json:"keys"`
}

type RgwKey struct {
	User      string `json:"user"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func random(min int, max int) int {
	rand.Seed(time.Now().UnixNano())
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
	// no bucket can created on this user, should not export
	if userData.MaxBuckets == -1 {
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

func updateNfsExport(uid string) {
	output, err := sh.Command("radosgw-admin", "user", "info", "--uid", uid).Output()
	if err != nil {
		fmt.Println("Can not get user info for uid", uid)
		return
	}
	var userData RgwUser
	err = json.Unmarshal(output, &userData)
	if err != nil {
		fmt.Println("Can not parse user info output for uid", uid)
		return
	}
	if len(userData.Keys) <= 0 {
		fmt.Println("Not found any user keys for uid", uid)
		return
	}

	conn, ioctx := connect()
	defer ioctx.Destroy()
	defer conn.Shutdown()

	updateNfsExportObj(ioctx, &userData)
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

func loadExportTemplate(ioctx *rados.IOContext, exportTmplName string) string {
	stat, _ := ioctx.Stat(exportTmplName)
	size := stat.Size
	data := make([]byte, size)
	ioctx.Read(exportTmplName, data, 0)
	return string(data)
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
	s := strings.Replace(string(data), targetExport, "", 1)
	if len(s) == 0 {
		s = "\n"
	}
	ioctx.WriteFull(exportName, []byte(s))
	ioctx.Unlock(exportName, lock, cookie)
}

func generateExportId(ioctx *rados.IOContext, prefix string) int {
	availExportIds := make(map[string]bool)
	for i := 1; i <= 65535; i++ {
		exportId := fmt.Sprint(i)
		availExportIds[exportId] = true
	}

	// load existed export id
	ioctx.ListObjects(func(oid string) {
		if !strings.HasPrefix(oid, prefix) {
			return
		}
		usedExportId := make([]byte, 100)
		len, err := ioctx.GetXattr(oid, "export_id", usedExportId)
		if err != nil {
			return
		}
		delete(availExportIds, string(usedExportId[0:len]))
	})

	for exportId := range availExportIds {
		// just return first available export id
		i, _ := strconv.Atoi(exportId)
		return i
	}
	return -1
}

func loadExportId(ioctx *rados.IOContext, exportObjName string) int {
	data := make([]byte, 10)
	size, err := ioctx.GetXattr(exportObjName, "export_id", data)
	i, err := strconv.Atoi(string(data[:size]))
	if err != nil {
		return -1
	}
	return i
}

func createNfsExportObj(ioctx *rados.IOContext, data *RgwUser) string {
	userId := data.UserId
	accessKey := data.Keys[0].AccessKey
	secretKey := data.Keys[0].SecretKey
	displayName := data.DisplayName

	exportId := generateExportId(ioctx, "export_")

	exportTmplName := utils.GetEnv("NFS_EXPORT_TMPL", "export.tmpl")
	exportTmpl := loadExportTemplate(ioctx, exportTmplName)
	exportObjName := makeExportObjName(userId)
	export := fmt.Sprintf(exportTmpl, exportId, displayName, userId, accessKey, secretKey)
	ioctx.WriteFull(exportObjName, []byte(export))

	// put pseudo (export path) and export_id to xattr
	ioctx.SetXattr(exportObjName, "pseudo", []byte(displayName))
	ioctx.SetXattr(exportObjName, "export_id", []byte(fmt.Sprint(exportId)))
	return exportObjName
}

func updateNfsExportObj(ioctx *rados.IOContext, data *RgwUser) {
	uid := data.UserId
	user := data.Keys[0].User
	accessKey := data.Keys[0].AccessKey
	secretKey := data.Keys[0].SecretKey
	displayName := data.DisplayName

	// loading export obj template
	exportTmplName := utils.GetEnv("NFS_EXPORT_TMPL", "export.tmpl")
	exportTmpl := loadExportTemplate(ioctx, exportTmplName)

	// laoding export id
	exportObjName := makeExportObjName(uid)
	exportId := loadExportId(ioctx, exportObjName)

	// generate export obj content and write
	content := fmt.Sprintf(exportTmpl, exportId, displayName, user, accessKey, secretKey)
	ioctx.WriteFull(exportObjName, []byte(content))

	// put pseudo (export path) and export to xattr
	ioctx.SetXattr(exportObjName, "pseudo", []byte(displayName))
	ioctx.SetXattr(exportObjName, "export_id", []byte(fmt.Sprint(exportId)))
}

func removeNfsExportObj(ioctx *rados.IOContext, exportObjName string) {
	ioctx.Delete(exportObjName)
}

func HandleNfsExport(req *http.Request, body []byte) {
	_, isSubuser := req.URL.Query()["subuser"]
	_, isKey := req.URL.Query()["key"]
	_, isQuota := req.URL.Query()["quota"]
	_, isCaps := req.URL.Query()["caps"]

	if isSubuser || isCaps || isQuota || isKey {
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

func setupPermission(parentHandle rgw.RgwFileHandle, path string) {
	// take current target name
	index := strings.Index(path, "/")
	if index == -1 {
		index = len(path)
	}
	target := path[0:index]

	// load target handle
	handle, _ := parentHandle.Lookup(target)
	parentAttr := parentHandle.GetAttr()
	attr := handle.GetAttr()

	// inherit parent's permission if not initailzed
	if !attr.IsInitialized() {
		// inherit uid and gid from parent
		attrMap := map[string]uint{
			"uid": parentAttr.Uid,
			"gid": parentAttr.Gid,
		}
		// inherit permission when directory only
		if attr.IsDir() {
			attrMap["mode"] = (attr.Mode & 0777000) | (parentAttr.Mode & 0777)
		}
		handle.SetAttr(attrMap)
	}
	//fmt.Println("is dir? ", attr.IsDir())
	//fmt.Printf("%d %d %o\n", attr.Uid, attr.Gid, attr.Mode)

	// setup child permission
	if index < len(path) {
		setupPermission(handle, path[index+1:])
	}
	handle.Release()
}

func InheritNfsPermission(request *http.Request) {
	// check auth
	accessKey := ExtractAccessKey(request)
	if accessKey == "" {
		return
	}
	uid, cred, s3Err := cmd.GetCredentials(accessKey)
	if s3Err != cmd.ErrNone {
		return
	}

	//cephUser := utils.GetEnv("NFS_CONFIG_User", "admin")
	//userArg := fmt.Sprintf("--name=client.%s", cephUser)
	args := []string{"kaoliang", "--conf=/etc/ceph/ceph.conf", "--name=client.admin", "--cluster=ceph"}
	radosgw := rgw.Create(args)
	rgwfs, _ := rgw.Mount(radosgw, uid, cred.AccessKey, cred.SecretKey)
	defer rgw.Umount(rgwfs)

	path := strings.TrimPrefix(request.URL.Path, "/")
	index := strings.Index(path, "/")

	if index == -1 { // only bucket in path
		return
	}

	bucket := path[0:index]
	rootHandle := rgw.MakeRgwFileHandle(rgwfs)
	defer rootHandle.Release()
	bh, _ := rootHandle.Lookup(bucket)
	defer bh.Release()

	setupPermission(bh, path[index+1:])
}

func getValue(q url.Values, key string, base int) (uint, error) {
	s := q[key][0]
	i, err := strconv.ParseUint(s, base, 32)
	if err != nil {
		return 0, err
	}
	return uint(i), nil
}

func PatchBucketPermission(c *gin.Context) {
	// extract access key
	accessKey := ExtractAccessKey(c.Request)
	if accessKey == "" {
		writeErrorResponse(c, cmd.ErrInvalidAccessKeyID)
		return
	}
	// get s3 user id and secret key
	name, cred, s3Err := cmd.GetCredentials(accessKey)
	if s3Err != cmd.ErrNone {
		writeErrorResponse(c, s3Err)
		return
	}
	// connect ceph
	args := []string{"kaoliang", "--conf=/etc/ceph/ceph.conf", "--name=client.admin", "--cluster=ceph"}
	radosgw := rgw.Create(args)
	rgwfs, cephErr := rgw.Mount(radosgw, name, cred.AccessKey, cred.SecretKey)
	defer rgw.Umount(rgwfs)

	rootHandle := rgw.MakeRgwFileHandle(rgwfs)
	defer rootHandle.Release()

	// load bucket file handle
	bucket := c.Param("bucket")
	bh, cephErr := rootHandle.Lookup(bucket)
	if cephErr != nil && cephErr == rgw.CephError(-2) {
		writeErrorResponse(c, cmd.ErrNoSuchBucket)
		return
	}
	defer bh.Release()

	attrMap := make(map[string]uint)
	q := c.Request.URL.Query()
	// extract uid (nfs)
	if len(q["uid"]) != 0 {
		uid, err := getValue(q, "uid", 10)
		if err != nil {
			writeErrorResponse(c, cmd.ErrInvalidRequest)
			return
		}
		attrMap["uid"] = uid
	}
	// extract gid
	if len(q["gid"]) != 0 {
		gid, err := getValue(q, "gid", 10)
		if err != nil {
			writeErrorResponse(c, cmd.ErrInvalidRequest)
			return
		}
		attrMap["gid"] = gid
	}
	// extract mode
	if len(q["mode"]) != 0 {
		mode, err := getValue(q, "mode", 8)
		if err != nil || mode < 0 || mode > 0777 {
			writeErrorResponse(c, cmd.ErrInvalidRequest)
			return
		}
		attrMap["mode"] = mode
	}
	// set new attribure
	bh.SetAttr(attrMap)
}
