// +build linux
// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"fmt"
	"log"
	"os/exec"
)

type HealthExt struct {
	Director   []string
	ExtCommand string
}

func NewHealthExt(cmd string) *HealthExt {
	dir := make([]string, 0, MaxBackends)
	return &HealthExt{dir, cmd}
}

func (h *HealthExt) AddDirector(backend string) error {
	h.Director = append(h.Director, backend)
	return fmt.Errorf("Error to add backend %s\n", backend)
}

func extProbe(cmd, addr string) error {
	return exec.Command(cmd, addr).Run()
}

// check the backend status
func (t *HealthExt) BuildActiveBackends() (map[string]int, error) {
	backends := make(map[string]int, MaxBackends)

	if len(t.Director) == 0 {
		return backends, fmt.Errorf("Empty directory server list\n")
	}

	type backendStatus struct {
		backend string
		err     error
	}

	results := make(chan backendStatus, MaxBackends)

	probe := func(cmd, addr string) {
		err := extProbe(cmd, addr)
		results <- backendStatus{addr, err}
	}

	numWorkers := 0
	for _, addr := range t.Director {
		go probe(t.ExtCommand, addr)
		numWorkers++
	}
	for i := 0; i < numWorkers; i++ {
		r := <-results
		if r.err == nil {
			backends[r.backend] = FlagUp
			//log.Printf("host: %s\n", r.backend)
		} else {
			log.Printf("ext error: %s", r.err)
		}
	}
	//log.Printf("Active server: %v\n", backends)
	return backends, nil
}
