// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"flag"
	"github.com/zhgwenming/gbalancer/config"
	logger "github.com/zhgwenming/gbalancer/log"
	"net"
	"sync"
)

var (
	log       = logger.NewLogger()
	useTunnel = flag.Bool("tunnel", true, "use tunnel mode")
	failover  = flag.Bool("failover", false, "whether to enable failover mode for scheduling")
)

func Serve(settings *config.Configuration, wgroup *sync.WaitGroup, done chan struct{}, status chan map[string]int) {
	job := make(chan *Request)

	// start the scheduler
	sch := NewScheduler(*failover, *useTunnel)
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
					req := &Request{Conn: conn}
					job <- req
				} else {
					if neterr, ok := err.(net.Error); ok && neterr.Temporary() {
						log.Printf("%s\n", err)
					} else {
						// we should got a errClosing
						log.Printf("stop listening for %s:%s\n", listenAddr.Net, listenAddr.Addr)
						return
					}
				}
			}
		}()
	}
}
