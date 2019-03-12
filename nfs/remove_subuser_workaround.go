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

package main

import (
	"encoding/json"
	"fmt"
	"github.com/ceph/go-ceph/rados"
	sh "github.com/codeskyblue/go-sh"
	"regexp"
	"strconv"
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

func main() {

	// connect ceph cluster
	conn, _ := rados.NewConnWithUser("admin")
	conn.ReadDefaultConfigFile()
	conn.Connect()
	defer conn.Shutdown()
	ioctx, _ := conn.OpenIOContext("nfs-ganesha")
	defer ioctx.Destroy()

	// loading export list
	oid := "export"
	stat, _ := ioctx.Stat(oid)
	size := stat.Size
	data := make([]byte, size)
	ioctx.Read(oid, data, 0)

	// get exported uids
	uids := make([]string, 0)
	pattern := regexp.MustCompile("%url \"rados://nfs-ganesha/export_([0-9a-zA-Z-]+)\"")
	match := pattern.FindAllStringSubmatch(string(data), -1)
	for i := range match {
		uids = append(uids, match[i][1])
	}

	// loading export template
	tmplName := "export.tmpl"
	stat, _ = ioctx.Stat(tmplName)
	size = stat.Size
	tmpl := make([]byte, size)
	ioctx.Read(tmplName, tmpl, 0)

	for _, uid := range uids {
		// loading ganesha export config
		export := "export_" + uid
		stat, _ := ioctx.Stat(export)
		size := stat.Size
		data := make([]byte, size)
		ioctx.Read(export, data, 0)

		// check uid is main user or not
		uidPattern := regexp.MustCompile("User_Id *= *\"([0-9A-Za-z-:]+)\";")
		uidMatch := uidPattern.FindStringSubmatch(string(data))
		exportUser := uidMatch[1]

		// only update ganesha config which use subuser
		if uid == exportUser {
			continue
		}

		fmt.Println("updating ganesha config:", uid)

		// get export id
		pattern = regexp.MustCompile("Export_ID *= *([0-9]+);")
		match := pattern.FindStringSubmatch(string(data))
		exportId, _ := strconv.Atoi(match[1])

		// loading main user's info
		output, err := sh.Command("radosgw-admin", "user", "info", "--uid", uid).Output()
		if err != nil {
			fmt.Println("Can not get user info for uid", uid)
			continue
		}
		var userData RgwUser
		err = json.Unmarshal(output, &userData)
		if err != nil {
			fmt.Println("Can not parse user info output for uid", uid)
			continue
		}
		if len(userData.Keys) <= 0 {
			fmt.Println("Not found any user keys for uid", uid)
			continue
		}

		// get main user's access key and secret key
		var accessKey, secretKey string
		for _, key := range userData.Keys {
			if key.User == uid {
				accessKey = key.AccessKey
				secretKey = key.SecretKey
				break
			}
		}
		displayName := userData.DisplayName
		updatedExport := fmt.Sprintf(string(tmpl), exportId, displayName, uid, accessKey, secretKey)

		ioctx.WriteFull(export, []byte(updatedExport))

		// put pseudo (export path) and export to xattr
		ioctx.SetXattr(export, "pseudo", []byte(displayName))
		ioctx.SetXattr(export, "export_id", []byte(fmt.Sprint(exportId)))
	}
}
