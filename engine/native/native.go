// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"flag"
	"github.com/zhgwenming/gbalancer/config"
	logger "github.com/zhgwenming/gbalancer/log"
	"net"
	"runtime/debug"
	"sync"
)

var (
	tunnels    = flag.Uint("tunnels", 0, "number of tunnels per server")
	streamPort = flag.String("streamport", "6900", "port of the remote stream server")
	failover   = flag.Bool("failover", false, "whether to enable failover mode for scheduling")
	shuffle    = flag.Bool("shuffle", true, "whether to enable shuffle for server list")
)

func Serve(settings *config.Configuration, wgroup *sync.WaitGroup, done chan struct{}, status chan map[string]int) {
	job := make(chan *Request)

	// start the scheduler
	sch := NewScheduler(*failover, *tunnels)
	go sch.EventLoop(job, status)

	listenAddrs, err := settings.GetListenAddrs()
	if err != nil {
		logger.GlobalLog.Fatal(err)
	}

	for _, listenAddr := range listenAddrs {
		listener, err := listenAddr.Listen()

		// close the listener makes the unix socket file got removed
		wgroup.Add(1)
		go func() {
			<-done
			listener.Close()
		}()

		if err != nil {
			logger.GlobalLog.Fatal(err)
		}

		// tcp/unix listener
		go func(listen config.ListenAddr) {

			for {
				if conn, err := listener.Accept(); err == nil {
					req := &Request{Conn: conn}
					job <- req
				} else {
					if neterr, ok := err.(net.Error); ok && neterr.Temporary() {
						logger.GlobalLog.Printf("%s\n", err)
					} else {
						// we should got a errClosing
						logger.GlobalLog.Printf("stop listening for %s:%s\n", listen.Net, listen.Addr)
						wgroup.Done()
						return
					}
				}
			}
		}(listenAddr)
	}
}

func RecoverReport() {
	if p := recover(); p != nil {
		logger.GlobalLog.Printf("%s\nbacktrace:\n%s", p, debug.Stack())
	}
}
