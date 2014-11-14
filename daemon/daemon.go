// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package daemon

import (
	"flag"
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/utils"
	//"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

const (
	DAEMON_ENV = "__GO_DAEMON_MODE"
)

var (
	log        = logger.NewLogger()
	daemonMode = flag.Bool("daemon", false, "daemon mode")
	pidFile    = flag.String("pidfile", "", "pid file")
	sigChan    = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM)

	if !*daemonMode {
		return
	}

	if _, child := syscall.Getenv(DAEMON_ENV); child {
		syscall.Unsetenv(DAEMON_ENV)
	} else {
		err := syscall.Setenv(DAEMON_ENV, "")
		if err != nil {
			log.Fatal(err)
		}

		syscall.Setsid()

		cmd := exec.Command(os.Args[0], os.Args...)
		if err = cmd.Start(); err == nil {
			log.Printf("Started daemon as pid %s\n", cmd.Process.Pid)
			os.Exit(0)
		} else {
			log.Printf("error to run in daemon mode - %s", err)
			os.Exit(1)
		}
	}

	os.Chdir("/")
}

func CreatePidfile() {
	if *pidFile != "" {
		if err := utils.WritePid(*pidFile); err != nil {
			fmt.Printf("error: %s\n", err)
			log.Fatal("error:", err)
		}
	}
}

func RemovePidfile() {
	if *pidFile != "" {
		if err := os.Remove(*pidFile); err != nil {
			log.Printf("error to remove pidfile %s:", err)
		}
	}
}

func WaitSignal(cleanup func()) {
	// waiting for exit signals
	for sig := range sigChan {
		log.Printf("captured %v, exiting..", sig)
		// exit if we get any signal
		// Todo - catch signal other than SIGTERM/SIGINT
		break
	}

	cleanup()
	RemovePidfile()
	return
}
