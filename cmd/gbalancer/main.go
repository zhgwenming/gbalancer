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
	VERSION = "0.6.3"
)

var (
	wgroup       = &sync.WaitGroup{}
	log          = logger.NewLogger()
	configFile   = flag.String("config", "gbalancer.json", "Configuration file")
	printVersion = flag.Bool("version", false, "print gbalancer version")
)

func PrintVersion() {
	fmt.Printf("gbalancer version: %s\n", VERSION)
	os.Exit(0)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	if *printVersion {
		PrintVersion()
	}

	// Load configurations
	settings, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		log.Fatal("error:", err)
	}
	log.Printf(settings.ListenInfo())

	daemon.CreatePidfile()

	// create the service goroutine
	done := engine.Serve(settings, wgroup)

	// wait the exit signal then do cleanup
	daemon.WaitSignal(func() {
		close(done)
		wgroup.Wait()
	})
}
