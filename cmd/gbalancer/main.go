// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"fmt"
	"github.com/zhgwenming/gbalancer/config"
	"github.com/zhgwenming/gbalancer/daemon"
	"github.com/zhgwenming/gbalancer/engine"
	logger "github.com/zhgwenming/gbalancer/log"
	"os"
	"runtime"
	"sync"
)

const (
	VERSION = "0.6.5"
)

var (
	wgroup       = &sync.WaitGroup{}
	configFile   = flag.String("config", "gbalancer.json", "Configuration file")
	daemonMode   = flag.Bool("daemon", false, "daemon mode")
	printVersion = flag.Bool("version", false, "print gbalancer version")
	logdir 	     = flag.String("logdir", "", "Custom directory and filename")
)

func PrintVersion() {
	fmt.Printf("gbalancer version: %s\n", VERSION)
	os.Exit(0)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	logger.GlobalLog = logger.NewLogger(*logdir)

	if *printVersion {
		PrintVersion()
	}

	if *daemonMode {
		os.Chdir("/")
	}

	// Load configurations
	settings, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		logger.GlobalLog.Fatal("error:", err)
	}
	logger.GlobalLog.Printf(settings.ListenInfo())

	daemon.CreatePidfile()

	// create the service goroutine
	done := engine.Serve(settings, wgroup)

	// wait the exit signal then do cleanup
	daemon.WaitSignal(func() {
		close(done)
		wgroup.Wait()
	})
}
