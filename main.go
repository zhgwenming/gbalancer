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

var (
	VERSION string
)

var (
	wgroup       = &sync.WaitGroup{}
	configFile   = flag.String("config", "/etc/gbalancer/gbalancer.json", "Configuration file")
	printVersion = flag.Bool("version", false, "print gbalancer version")

	daemonMode = flag.Bool("daemon", false, "daemon mode")
	pidFile    = flag.String("pidfile", "", "pid file")
	logdir 	   = flag.String("logdir", "", "Custom directory and filename")
)

func PrintVersion() {
	fmt.Printf("gbalancer version: %s\n", VERSION)
	os.Exit(0)
}

type Server struct {
	settings *config.Configuration
	wgroup   *sync.WaitGroup
	done     chan struct{}
}

func (s *Server) Serve() {
	// create the service goroutine
	s.done = engine.Serve(s.settings, s.wgroup)
}

func (s *Server) Stop() {
	close(s.done)
	s.wgroup.Wait()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	logger.GlobalLog = logger.NewLogger(*logdir)

	if *printVersion {
		PrintVersion()
	}

	if *daemonMode {
		if err := config.CheckFile(*configFile); err != nil {
			fmt.Println(err)
			logger.GlobalLog.Fatal(err)
		}
	}

	// Load configurations
	settings, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		logger.GlobalLog.Fatal("error:", err)
	}
	logger.GlobalLog.Printf(settings.ListenInfo())

	srv := &Server{settings: settings, wgroup: wgroup}

	foreground := !*daemonMode
	n := nestor.Handle(*pidFile, foreground, srv)

	if err := nestor.Start(n); err != nil {
		logger.GlobalLog.Fatal(err)
	}
}
