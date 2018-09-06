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

func main() {
	pid := findPidByName("ganesha.nfsd")
	if pid == -1 {
		return
	}
	process, _ := os.FindProcess(pid)
	err = process.Signal(syscall.SIGHUP)
	if err != nil {
		log.Fatal(err)
	}
}
