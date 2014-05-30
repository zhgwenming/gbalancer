// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"encoding/json"
	"flag"
	"log/syslog"
	"net"
	"os"
	"os/signal"
)

type Configuration struct {
	Service    string
	ExtCommand string
	User       string
	Pass       string
	Addr       string
	Port       string
	Backend    []string
}

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
	sigChan    = make(chan os.Signal, 1)
	configFile = flag.String("config", "gbalancer.json", "Configuration file")
	daemonMode = flag.Bool("daemon", false, "daemon mode")
	ipvsMode   = flag.Bool("ipvs", false, "to use lvs as loadbalancer")
	slog, _    = syslog.NewLogger(syslog.LOG_NOTICE, 0)
)

func init() {
	//if slog, err := syslog.NewLogger(syslog.LOG_NOTICE, log.LstdFlags); err != nil {
	//	log.Printf("Can't open logger")
	//	os.Exit(1)
	//}
	signal.Notify(sigChan, os.Interrupt)
}

func main() {
	flag.Parse()

	file, _ := os.Open(*configFile)

	if *daemonMode {
		os.Chdir("/")
	}

	decoder := json.NewDecoder(file)
	config := Configuration{
		Service: "galera",
		Addr:    "127.0.0.1",
		Port:    "3306",
	}

	err := decoder.Decode(&config)
	if err != nil {
		slog.Println("error:", err)
	}
	//slog.Printf("%v", config)
	slog.Printf("Listen on %s:%s, backend: %v", config.Addr, config.Port, config.Backend)

	tcpAddr := config.Addr + ":" + config.Port

	status := make(chan map[string]int, MaxBackends)
	//status := make(chan *BEStatus)

	// start the wrangler
	wgl := NewWrangler(config, status)

	go wgl.Monitor()

	if *ipvsMode {
		ipvs := NewIPvs(config.Addr, config.Port, "wlc")
		go ipvs.schedule(status)
	} else {
		listener, err := net.Listen("tcp", tcpAddr)

		if err != nil {
			slog.Fatal(err)
		}

		job := make(chan *Request)

		// start the scheduler
		sch := NewScheduler()
		go sch.schedule(job, status)

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					slog.Printf("%s\n", err)
				}
				//slog.Println("main: got a connection")
				req := &Request{conn: conn}
				job <- req
			}
		}()
	}
	for sig := range sigChan {
		slog.Printf("captured %v, exiting..", sig)
		return
	}

}
