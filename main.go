// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"fmt"
	"github.com/zhgwenming/gbalancer/config"
	"github.com/zhgwenming/gbalancer/engine"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/utils"
	"github.com/zhgwenming/gbalancer/wrangler"
	"net"
	"os"
	"os/signal"
	"runtime"
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
	wgroup       = &sync.WaitGroup{}
	log          = logger.NewLogger()
	sigChan      = make(chan os.Signal, 1)
	configFile   = flag.String("config", "gbalancer.json", "Configuration file")
	pidFile      = flag.String("pidfile", "", "pid file")
	failover     = flag.Bool("failover", false, "whether to enable failover mode for scheduling")
	daemonMode   = flag.Bool("daemon", false, "daemon mode")
	ipvsMode     = flag.Bool("ipvs", false, "to use lvs as loadbalancer")
	ipvsRemote   = flag.Bool("remote", false, "independent director")
	useTunnel    = flag.Bool("tunnel", true, "use tunnel mode")
	printVersion = flag.Bool("version", false, "print gbalancer version")
)

func init() {
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
}

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

	if *pidFile != "" {
		if err = utils.WritePid(*pidFile); err != nil {
			fmt.Printf("error: %s\n", err)
			log.Fatal("error:", err)
		}
	}

	status := make(chan map[string]int, MaxBackends)
	//status := make(chan *BEStatus)

	// start the wrangler
	wgl := wrangler.NewWrangler(settings, status)

	go wgl.Monitor()

	done := make(chan struct{})
	if *ipvsMode {
		wgroup.Add(1)
		if *ipvsRemote {
			ipvs := engine.NewIPvs(settings.Addr, settings.Port, "wlc", done, wgroup)
			go ipvs.RemoteSchedule(status)
		} else {
			//ipvs := NewIPvs(IPvsLocalAddr, settings.Port, "sh", done)
			ipvs := engine.NewIPvs(IPvsLocalAddr, settings.Port, "wlc", done, wgroup)
			go ipvs.LocalSchedule(status)
		}
	} else {
		job := make(chan *Request)

		// start the scheduler
		sch := NewScheduler(*failover)
		go sch.schedule(job, status)

		listenAddrs, err := settings.GetListenAddrs()
		if err != nil {
			log.Fatal(err)
		}

		for _, listenAddr := range listenAddrs {
			listener, err := listenAddr.Listen()

			// close the listener makes the unix socket file got removed
			wgroup.Add(1)
			go func() {
				<-done
				listener.Close()
				wgroup.Done()
			}()

			if err != nil {
				log.Fatal(err)
			}

			// tcp/unix listener
			go func() {
				for {
					if conn, err := listener.Accept(); err == nil {
						//log.Println("main: got a connection")
						req := &Request{conn: conn}
						job <- req
					} else {
						if neterr, ok := err.(net.Error); ok && neterr.Temporary() {
							log.Printf("%s\n", err)
						} else {
							// we should got a errClosing
							log.Printf("Existing listen loop\n")
							return
						}
					}
				}
			}()
		}
	}

	// waiting for exit signals
	for sig := range sigChan {
		log.Printf("captured %v, exiting..", sig)
		close(done)
		wgroup.Wait()

		// remove pid file
		if *pidFile != "" {
			if err = os.Remove(*pidFile); err != nil {
				log.Printf("error to remove pidfile %s:", err)
			}
		}
		return
	}

}
