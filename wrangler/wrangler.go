// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package wrangler

import (
	"github.com/zhgwenming/gbalancer/config"
	logger "github.com/zhgwenming/gbalancer/log"
	"os"
	"time"
)

//var (
//	log = logger.NewLogger()
//)

type healthDriver interface {
	AddDirector(backend string) error
	BuildActiveBackends() (map[string]int, error)
}

type Wrangler struct {
	healthExec healthDriver
	Backends   map[string]int
	BackChan   chan<- map[string]int
}

func NewWrangler(config *config.Configuration, back chan<- map[string]int) *Wrangler {
	var hexec healthDriver
	switch config.Service {
	case "galera":
		hexec = NewGalera(config.User, config.Pass, config.Timeout)
	case "tcp":
		hexec = NewHealthTcp()
	case "http":
		hexec = NewHealthHTTP()
	case "ext":
		hexec = NewHealthExt(config.ExtCommand)
		if config.ExtCommand == "" {
			logger.GlobalLog.Printf("Need to specify ExtCommand for ext Service")
			os.Exit(1)
		}
	default:
		logger.GlobalLog.Printf("Unknown healthy monitor: %s", config.Service)
		os.Exit(1)
	}

	backends := make(map[string]int, MaxBackends)
	w := &Wrangler{hexec, backends, back}
	for _, b := range config.Backend {
		hexec.AddDirector(b)
	}
	return w
}

func (w *Wrangler) ValidBackends() {
	backends, err := w.healthExec.BuildActiveBackends()
	if err != nil {
		logger.GlobalLog.Printf("wrangler: %s\n", err)
		return
	}

	//logger.GlobalLog.Printf("backends is %v\n", backends)

	// remove fail node from w.Backends first
	for b := range w.Backends {
		if _, ok := backends[b]; !ok {
			delete(w.Backends, b)
			logger.GlobalLog.Printf("wrangler: detected server %s is down\n", b)
		}
	}

	// add new backends
	for b := range backends {
		if _, ok := w.Backends[b]; !ok {
			logger.GlobalLog.Printf("wrangler: detected server %s is up\n", b)
			w.Backends[b] = backends[b]
		}
	}

	// full set of backends needed by the ipvs engine
	if len(backends) > 0 {
		w.BackChan <- backends
	}
}

func (w *Wrangler) Monitor() {
	for {
		w.ValidBackends()
		if len(w.Backends) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// periodic check
	ticker := time.NewTicker(CheckInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			//logger.GlobalLog.Printf("got a tick")
			w.ValidBackends()
		}
	}
}
