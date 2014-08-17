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
	VERSION = "0.5.3"
)

var (
	wgroup       = &sync.WaitGroup{}
	log          = logger.NewLogger()
	configFile   = flag.String("config", "gbalancer.json", "Configuration file")
	daemonMode   = flag.Bool("daemon", false, "daemon mode")
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

	if *daemonMode {
		os.Chdir("/")
	}

	settings, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		log.Fatal("error:", err)
	}

	// for compatible reason, may remove in the future
	if settings.Addr != "" {
		tcpAddr := "tcp://" + settings.Addr + ":" + settings.Port
		settings.AddListen(tcpAddr)
	}

	//log.Printf("%v", config)
	log.Printf(settings.ListenInfo())
	daemon.CreatePidfile()

	done := engine.Serve(settings, wgroup)

	// wait the exit signal then do cleanup
	daemon.WaitSignal(func() {
		close(done)
		wgroup.Wait()
	})
}
