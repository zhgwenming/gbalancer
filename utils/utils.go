// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package utils

import (
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

var (
	log = logger.NewLogger()
)

func RunCommand(cmd string) error {
	args := strings.Fields(cmd)
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Err: %s Output: %s, Cmd %s", err, output, cmd)
		log.Printf("%s", err)
	}
	return err
}

func EnsureCommands(cmds []string) error {
	for _, c := range cmds {
		if err := RunCommand(c); err != nil {
			return err
		}
	}
	return nil
}

func GetFirstIPAddr() (addr string) {
	addrs, _ := net.InterfaceAddrs()
	for _, i := range addrs {
		ipnet, ok := i.(*net.IPNet)

		if !ok {
			log.Fatal("assertion err:", i)
		}

		ip4 := ipnet.IP.To4()

		if !ip4.IsLoopback() && ip4 != nil {
			addr = ip4.String()
			break
		}
	}
	//log.Printf("%v", addr)
	return
}

func GetIPAddrs() (addresses []string) {
	addrs, _ := net.InterfaceAddrs()
	for _, i := range addrs {
		ipnet, ok := i.(*net.IPNet)

		if !ok {
			log.Fatal("assertion err: ", i)
		}

		ip4 := ipnet.IP.To4()

		if !ip4.IsLoopback() && ip4 != nil {
			addr := ip4.String()
			addresses = append(addresses, addr)
			break
		}
	}
	//log.Printf("%v", addresses)
	return
}

func Shuffle(src []string) []string {
	length := len(src)

	dst := make([]string, length)
	perm := rand.Perm(length)

	for i, v := range perm {
		dst[v] = src[i]
	}

	return dst
}

func WritePid(pidfile string) error {
	var file *os.File

	if _, err := os.Stat(pidfile); os.IsNotExist(err) {
		if file, err = os.Create(pidfile); err != nil {
			return err
		}
	} else {
		if file, err = os.OpenFile(pidfile, os.O_RDWR, 0); err != nil {
			return err
		}
		pidstr := make([]byte, 8)

		n, err := file.Read(pidstr)
		if err != nil {
			return err
		}

		if n > 0 {
			pid, err := strconv.Atoi(string(pidstr[:n]))
			if err != nil {
				fmt.Printf("err: %s, overwriting pidfile", err)
			}

			process, _ := os.FindProcess(pid)
			if err = process.Signal(syscall.Signal(0)); err == nil {
				return fmt.Errorf("pid: %d is running", pid)
			} else {
				fmt.Printf("err: %s, cleanup pidfile", err)
			}

			if file, err = os.Create(pidfile); err != nil {
				return err
			}

		}

	}
	defer file.Close()

	pid := strconv.Itoa(os.Getpid())
	fmt.Fprintf(file, "%s", pid)
	return nil
}
