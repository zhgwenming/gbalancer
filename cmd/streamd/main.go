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
			logger.GlobalLog.Printf("error: %s", err)
			os.Exit(1)
		} else {
			logger.GlobalLog.Printf("Test_Issue: pidFile is correct\n")
		}
		defer func() {
			if err := os.Remove(*pidFile); err != nil {
				logger.GlobalLog.Printf("error to remove pidfile %s:", err)
			} else {
				logger.GlobalLog.Printf("Test_Issue: remove is called successfully\n")
			}
		}()
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Printf("Listen error: %s\n", err)
		logger.GlobalLog.Printf("Listen error: %s", err)
		os.Exit(1)
	} else {
		logger.GlobalLog.Printf("Test_Issue: listener is Correct\n")
	}
	
	var spdyConns = make([] *spdystream.Connection, 128)
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.GlobalLog.Printf("Accept error: %s", err)
			} else {
				logger.GlobalLog.Printf("Test_Issue: Accept is called successfully\n")
			}
			spdyConn, err := spdystream.NewConnection(conn, true)
			if err != nil {
				conn.Close()
				logger.GlobalLog.Printf("New spdyConnection error, %s", err)
			} else {
				logger.GlobalLog.Printf("Test_Issue: spdyConnection is called successfully\n")
			}
			spdyConns = append(spdyConns, spdyConn)
			go spdyConn.Serve(AgentStreamHandler)
		}
	}()
	
	fmt.Printf("starting clean up connections...")
	
	// waiting for exit signals
	for sig := range sigChan {
		logger.GlobalLog.Printf("captured %v, exiting..", sig)
		
		for _, spdyConn := range spdyConns {
			if nil != spdyConn{
				spdyConn.Close()
			}
		}
		return
	}
}
