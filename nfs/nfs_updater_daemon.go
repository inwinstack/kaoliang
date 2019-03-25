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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/godbus/dbus"
)

func findPidByName(name string) int {
	cmd := exec.Command("ps", "ax", "-o", "pid,cmd")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return -1
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
		return nil
	}
	// as dbus-send
	bo := conn.Object("org.ganesha.nfsd", "/org/ganesha/nfsd/ExportMgr")
	call := bo.Call("org.ganesha.nfsd.exportmgr.ShowExports", 0)
	if call.Err != nil {
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
		displayName := make([]byte, 100)
		len, err := ioctx.GetXattr(oid, "pseudo", displayName)
		if err != nil {
			return
		}
		path := "/" + string(displayName[0:len])
		configured[path] = true
	})
	return configured
}

func isExists(target string, paths map[string]bool) bool {
	_, found := paths[target]
	return found
}

func getExport(userId, poolName, target string) []byte {
	conn, _ := rados.NewConnWithUser(userId)
	conn.ReadDefaultConfigFile()
	conn.Connect()
	defer conn.Shutdown()
	ioctx, _ := conn.OpenIOContext(poolName)
	defer ioctx.Destroy()

	var offset uint64 = 0
	buffer := make([]byte, 50)
	data := make([]byte, 0)
	for {
		ret, _ := ioctx.Read(target, buffer, offset)
		data = append(data, buffer[0:ret]...)
		if ret == len(buffer) {
			offset = offset + uint64(len(buffer))
		} else {
			break
		}
	}
	return data
}

func disableExport(export Export) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return
	}
	// as dbus-send
	bo := conn.Object("org.ganesha.nfsd", "/org/ganesha/nfsd/ExportMgr")
	call := bo.Call("org.ganesha.nfsd.exportmgr.RemoveExport", 0, export.Id)
	if call.Err != nil {
		return
	}
}

func reloadExport(pid int) {
	// send signal to process
	process, _ := os.FindProcess(pid)
	err := process.Signal(syscall.SIGHUP)
	if err != nil {
		return
	}
}

func needToUpdate(objectMtime int64, exportPath string) bool {
	// check local export list is exists
	fi, err := os.Stat(exportPath)
	if os.IsNotExist(err) {
		return true
	}

	mtime := fi.ModTime().Unix()
	return objectMtime > mtime
}

func execute(user, poolname, exportPath string) {
	// connect to pool
	conn, err := rados.NewConnWithUser(user)
	if err != nil {
		log.Fatal("Initialize rados connection failed: ", err)
	}
	err = conn.ReadDefaultConfigFile()
	if err != nil {
		log.Fatal("Read default config failed: ", err)
	}
	err = conn.Connect()
	if err != nil {
		log.Fatal("Rados connect failed: ", err)
	}
	defer conn.Shutdown()
	ioctx, err := conn.OpenIOContext(poolname)
	if err != nil {
		log.Fatal("Connect pool %s failed", poolname)
	}
	defer ioctx.Destroy()

	// take nfs-ganesha process id
	processName := "ganesha.nfsd"
	pid := findPidByName(processName)
	if pid == -1 {
		log.Printf("Process %s is not found", processName)
		return
	}
	// get the stats from export list object
	stat, err := ioctx.Stat("export")
	if err != nil {
		log.Printf("can not get the stats from export list object")
		return
	}
	// check export list object is modified or not
	if !needToUpdate(stat.ModTime.Unix(), exportPath) {
		return
	}

	log.Printf("update local export list")
	// update local export list
	data := getExport(user, poolname, "export")
	ioutil.WriteFile(exportPath, data, 0644)

	log.Printf("update the exports of nfs-ganesha")
	// send signal to reload exports (add only, no update and delete)
	reloadExport(pid)

	// load configured export path from rados
	paths := loadConfiguredPaths(user, poolname, "export_")

	// list enabled export on this host
	exports := listEnabledExport()
	if exports == nil {
		return
	}

	// disable export if not configured
	for _, export := range exports {
		if !isExists(export.Path, paths) {
			// disable export on this machine
			disableExport(export)
		}
	}
}

func main() {
	euid := os.Geteuid()
	if euid != 0 {
		fmt.Println("Permission denied, using root or sudo.")
		return
	}

	if len(os.Args) != 6 || os.Args[1] == "help" || os.Args[1] != "start" {
		fmt.Printf("Usage: %s [start|help] <ceph user> <pool name> <export path> <sleep milliseconds>\n", os.Args[0])
		return
	}

	user := os.Args[2]
	poolname := os.Args[3]
	exportPath := os.Args[4]
	sleepTimeMs, err := strconv.Atoi(os.Args[5])

	if err != nil || sleepTimeMs <= 0 {
		fmt.Printf("Usage: %s [start|help] <ceph user> <pool name> <export path> <sleep milliseconds>\n", os.Args[0])
		return
	}

	// setup signal channel
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	// exit if signal recieved
	go func() {
		s := <-sigs
		log.Printf("RECEIVED SIGNAL: %s", s)
		os.Exit(0)
	}()

	for {
		execute(user, poolname, exportPath)
		time.Sleep(time.Millisecond * time.Duration(sleepTimeMs))
	}
}
