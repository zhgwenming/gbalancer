// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package engine

import (
	"flag"
	"github.com/zhgwenming/gbalancer/config"
	"github.com/zhgwenming/gbalancer/engine/ipvs"
	"github.com/zhgwenming/gbalancer/engine/native"
//	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/wrangler"
	"sync"
)

var (
//	log        = logger.NewLogger()
	ipvsMode   = flag.Bool("ipvs", false, "to use lvs as loadbalancer")
	ipvsRemote = flag.Bool("remote", false, "independent director")
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
		native.Serve(settings, wgroup, done, status)
	}
	return done
}
