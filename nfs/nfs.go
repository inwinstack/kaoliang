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
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/ceph/go-ceph/rados"
	"github.com/godbus/dbus"
)

func findPidByName(name string) int {
	cmd := exec.Command("ps", "ax", "-o", "pid,cmd")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	processList := out.String()
	scanner := bufio.NewScanner(strings.NewReader(processList))
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if !strings.Contains(s, name) {
			continue
		}
		pid, _ := strconv.Atoi(strings.Split(s, " ")[0])
		return pid
	}
	return -1
}

type Export struct {
	Id   uint16
	Path string
}

func listEnabledExport() []Export {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
		return nil
	}
	// as dbus-send
	bo := conn.Object("org.ganesha.nfsd", "/org/ganesha/nfsd/ExportMgr")
	call := bo.Call("org.ganesha.nfsd.exportmgr.ShowExports", 0)
	if call.Err != nil {
		log.Fatal(err)
		return nil
	}
	exported := make([]Export, 0)
	result := (call.Body[1]).([][]interface{})
	for _, item := range result {
		id := (item[0]).(uint16)
		path := (item[1]).(string)
		if path == "/" { //root is alway exported
			continue
		}
		e := Export{id, path}
		exported = append(exported, e)
	}
	return exported
}

func loadConfiguredPaths(userId, poolName, prefix string) map[string]bool {
	conn, _ := rados.NewConnWithUser(userId)
	conn.ReadDefaultConfigFile()
	conn.Connect()
	defer conn.Shutdown()
	ioctx, _ := conn.OpenIOContext(poolName)
	defer ioctx.Destroy()

	configured := make(map[string]bool)
	ioctx.ListObjects(func(oid string) {
		if !strings.HasPrefix(oid, prefix) {
			return
		}
		path := "/" + strings.TrimPrefix(oid, prefix)
		configured[path] = true
	})
	return configured
}

func isExists(target string, paths map[string]bool) bool {
	_, found := paths[target]
	return found
}

func disableExport(export Export) {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	// as dbus-send
	bo := conn.Object("org.ganesha.nfsd", "/org/ganesha/nfsd/ExportMgr")
	call := bo.Call("org.ganesha.nfsd.exportmgr.RemoveExport", 0, export.Id)
	if call.Err != nil {
		log.Fatal(err)
	}
}

func reloadExport(pid int) {
	// send signal to process
	process, _ := os.FindProcess(pid)
	err := process.Signal(syscall.SIGHUP)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	euid := os.Geteuid()
	if euid != 0 {
		fmt.Println("Permission denied, using root or sudo.")
		return
	}

	if len(os.Args) != 4 || os.Args[1] == "help" || os.Args[1] != "start" {
		fmt.Printf("Usage: %s [start|help] <ceph user> <pool name>\n", os.Args[0])
		return
	}

	user := os.Args[2]
	poolname := os.Args[3]

	// take nfs-ganesha process id
	processName := "ganesha.nfsd"
	pid := findPidByName(processName)
	if pid == -1 {
		fmt.Printf("Process %s is not found\n", processName)
		return
	}

	// send signal to reload exports (add only, no update and delete)
	reloadExport(pid)

	// load configured export path from rados
	paths := loadConfiguredPaths(user, poolname, "export_")

	// list enabled export on this host
	exports := listEnabledExport()

	// disable export if not configured
	for _, export := range exports {
		if !isExists(export.Path, paths) {
			// disable export on this machine
			disableExport(export)
		}
	}
}
