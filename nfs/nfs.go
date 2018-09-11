package main

import (
	"bufio"
	"bytes"
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

func loadConfiguredPaths(userId, poolName string) map[string]bool {
	conn, _ := rados.NewConnWithUser(userId)
	conn.ReadDefaultConfigFile()
	conn.Connect()
	defer conn.Shutdown()
	ioctx, _ := conn.OpenIOContext(poolName)
	defer ioctx.Destroy()

	configured := make(map[string]bool)
	ioctx.ListObjects(func(oid string) {
		if oid == "export" {
			return
		}
		path := "/" + strings.Replace(oid, "export_", "", -1)
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

func reloadExport() {
	// take nfs-ganesha process id
	pid := findPidByName("ganesha.nfsd")
	if pid == -1 {
		return
	}
	// send signal to process
	process, _ := os.FindProcess(pid)
	err := process.Signal(syscall.SIGHUP)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	// send signal to reload exports (add only, no update and delete)
	reloadExport()

	// load configured export path from rados
	paths := loadConfiguredPaths("admin", "nfs-ganesha")

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
