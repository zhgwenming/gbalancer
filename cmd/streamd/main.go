// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"fmt"
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/utils"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

var (
	pidFile     = flag.String("pidfile", "", "pid file")
	listenAddr  = flag.String("listen", ":6900", "port number")
	serviceAddr = flag.String("to", "/var/lib/mysql/mysql.sock", "service address")
	log         = logger.NewLogger()
	sigChan     = make(chan os.Signal, 1)
	wgroup      = &sync.WaitGroup{}
)

func init() {
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()

	if *pidFile != "" {
		if err := utils.WritePid(*pidFile); err != nil {
			fmt.Printf("error: %s\n", err)
			log.Printf("error: %s", err)
			os.Exit(1)
		}
		defer func() {
			if err := os.Remove(*pidFile); err != nil {
				log.Printf("error to remove pidfile %s:", err)
			}
		}()
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Printf("Listen error: %s\n", err)
		log.Printf("Listen error: %s", err)
		os.Exit(1)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %s", err)
			}
			spdyConn, err := spdystream.NewConnection(conn, true)
			if err != nil {
				log.Printf("New spdyConnection error, %s", err)
			}
			go spdyConn.Serve(AgentStreamHandler)
		}
	}()

	// waiting for exit signals
	for sig := range sigChan {
		log.Printf("captured %v, exiting..", sig)

		return
	}
}
