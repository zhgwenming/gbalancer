// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package wrangler

import (
	logger "../log"
	"os"
	"time"
	"../config"
)

var (
	log        = logger.NewLogger()
)

type healthDriver interface {
	AddDirector(backend string) error
	BuildActiveBackends() (map[string]int, error)
}

type Wrangler struct {
	healthExec healthDriver
	Backends   map[string]int
	BackChan   chan<- map[string]int
}

func NewWrangler(config config.Configuration, back chan<- map[string]int) *Wrangler {
	var hexec healthDriver
	switch config.Service {
	case "galera":
		hexec = NewGalera(config.User, config.Pass)
	case "tcp":
		hexec = NewHealthTcp()
	case "http":
		hexec = NewHealthHTTP()
	case "ext":
		hexec = NewHealthExt(config.ExtCommand)
		if config.ExtCommand == "" {
			log.Printf("Need to specify ExtCommand for ext Service")
			os.Exit(1)
		}
	default:
		log.Printf("Unknown healthy monitor: %s", config.Service)
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
		log.Printf("wrangler: %s\n", err)
		return
	}

	//log.Printf("backends is %v\n", backends)

	// remove fail node first
	for b := range w.Backends {
		if _, ok := backends[b]; !ok {
			delete(w.Backends, b)
			log.Printf("wrangler: detected server %s is down\n", b)
		}
	}

	// add new backends
	for b := range backends {
		if _, ok := w.Backends[b]; !ok {
			log.Printf("wrangler: detected server %s is up\n", b)
			w.Backends[b] = backends[b]
		}
	}
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
			//log.Printf("got a tick")
			w.ValidBackends()
		}
	}
}
