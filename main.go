// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"./config"
	logger "./log"
	wrangler "./wrangler"
	"encoding/json"
	"flag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Request struct {
	conn    net.Conn
	backend *Backend
	err     error
}

type Forwarder struct {
	backend *Backend
	request *Request
	bytes   uint
}

var (
	wgroup     sync.WaitGroup
	log        = logger.NewLogger()
	sigChan    = make(chan os.Signal, 1)
	configFile = flag.String("config", "gbalancer.json", "Configuration file")
	daemonMode = flag.Bool("daemon", false, "daemon mode")
	ipvsMode   = flag.Bool("ipvs", false, "to use lvs as loadbalancer")
	ipvsRemote = flag.Bool("remote", false, "independent director")
)

func init() {
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
}

func main() {
	flag.Parse()

	file, _ := os.Open(*configFile)

	if *daemonMode {
		os.Chdir("/")
	}

	decoder := json.NewDecoder(file)
	config := config.Configuration{
		Service: "galera",
		Addr:    "127.0.0.1",
		Port:    "3306",
	}

	err := decoder.Decode(&config)
	if err != nil {
		log.Println("error:", err)
	}
	//log.Printf("%v", config)
	log.Printf("Listen on %s:%s, backend: %v", config.Addr, config.Port, config.Backend)

	tcpAddr := config.Addr + ":" + config.Port

	status := make(chan map[string]int, MaxBackends)
	//status := make(chan *BEStatus)

	// start the wrangler
	wgl := wrangler.NewWrangler(config, status)

	go wgl.Monitor()

	done := make(chan int, 1)
	if *ipvsMode {
		wgroup.Add(1)
		if *ipvsRemote {
			ipvs := NewIPvs(config.Addr, config.Port, "wlc", done)
			go ipvs.RemoteSchedule(status)
		} else {
			//ipvs := NewIPvs(IPvsLocalAddr, config.Port, "sh", done)
			ipvs := NewIPvs(IPvsLocalAddr, config.Port, "wlc", done)
			go ipvs.LocalSchedule(status)
		}
	} else {
		listener, err := net.Listen("tcp", tcpAddr)

		if err != nil {
			log.Fatal(err)
		}

		job := make(chan *Request)

		// start the scheduler
		sch := NewScheduler()
		go sch.schedule(job, status)

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Printf("%s\n", err)
				}
				//log.Println("main: got a connection")
				req := &Request{conn: conn}
				job <- req
			}
		}()
	}
	for sig := range sigChan {
		log.Printf("captured %v, exiting..", sig)
		done <- 1
		wgroup.Wait()
		return
	}

}
