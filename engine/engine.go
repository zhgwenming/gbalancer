// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package engine

import (
	"flag"
	"github.com/zhgwenming/gbalancer/config"
	"github.com/zhgwenming/gbalancer/engine/ipvs"
	"github.com/zhgwenming/gbalancer/engine/native"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/wrangler"
	"net"
	"sync"
)

var (
	log        = logger.NewLogger()
	ipvsMode   = flag.Bool("ipvs", false, "to use lvs as loadbalancer")
	ipvsRemote = flag.Bool("remote", false, "independent director")
	useTunnel  = flag.Bool("tunnel", true, "use tunnel mode")
	failover   = flag.Bool("failover", false, "whether to enable failover mode for scheduling")
)

func Serve(settings *config.Configuration, wgroup *sync.WaitGroup) (done chan struct{}) {
	status := make(chan map[string]int, native.MaxBackends)
	//status := make(chan *BEStatus)

	// start the wrangler
	wgl := wrangler.NewWrangler(settings, status)

	go wgl.Monitor()

	done = make(chan struct{})
	if *ipvsMode {
		wgroup.Add(1)
		if *ipvsRemote {
			ipvs := ipvs.NewIPvs(settings.Addr, settings.Port, "wlc", done, wgroup)
			go ipvs.RemoteSchedule(status)
		} else {
			//ipvs := NewIPvs(IPvsLocalAddr, settings.Port, "sh", done)
			ipvs := ipvs.NewIPvs(ipvs.IPvsLocalAddr, settings.Port, "wlc", done, wgroup)
			go ipvs.LocalSchedule(status)
		}
	} else {
		job := make(chan *native.Request)

		// start the scheduler
		sch := native.NewScheduler(*failover, *useTunnel)
		go sch.Schedule(job, status)

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
						req := &native.Request{Conn: conn}
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
	return done
}
