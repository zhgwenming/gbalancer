// +build linux
// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"fmt"
	"net/http"
)

type HealthHTTP struct {
	Director []string
}

func NewHealthHTTP() *HealthHTTP {
	dir := make([]string, 0, MaxBackends)
	return &HealthHTTP{dir}
}

func (h *HealthHTTP) AddDirector(backend string) error {
	h.Director = append(h.Director, backend)
	return fmt.Errorf("Error to add backend %s\n", backend)
}

func httpProbe(addr string) error {
	_, err := http.Get("http://" + addr + "/")
	return err
}

// check the backend status
func (t *HealthHTTP) BuildActiveBackends() (map[string]int, error) {
	backends := make(map[string]int, MaxBackends)

	if len(t.Director) == 0 {
		return backends, fmt.Errorf("Empty directory server list\n")
	}

	type backendStatus struct {
		backend string
		err     error
	}

	results := make(chan backendStatus, MaxBackends)

	probe := func(addr string) {
		err := httpProbe(addr)
		results <- backendStatus{addr, err}
	}

	numWorkers := 0
	for _, addr := range t.Director {
		go probe(addr)
		numWorkers++
	}
	for i := 0; i < numWorkers; i++ {
		r := <-results
		if r.err == nil {
			backends[r.backend] = FlagUp
			//slog.Printf("host: %s\n", r.backend)
		} else {
			slog.Printf("http error: %s", r.err)
		}
	}
	//slog.Printf("Active server: %v\n", backends)
	return backends, nil
}
