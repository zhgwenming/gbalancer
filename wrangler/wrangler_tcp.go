// +build linux darwin
// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package wrangler

import (
	"fmt"
	"net"
	"time"
)

type HealthTcp struct {
	Director []string
}

func NewHealthTcp() *HealthTcp {
	dir := make([]string, 0, MaxBackends)
	return &HealthTcp{dir}
}

func (c *HealthTcp) AddDirector(backend string) error {
	c.Director = append(c.Director, backend)
	return fmt.Errorf("Error to add backend %s\n", backend)
}

func tcpProbe(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		//log.Printf("%s\n", err)
		return err
	}
	defer conn.Close()
	return err
}

// check the backend status
func (t *HealthTcp) BuildActiveBackends() (map[string]int, error) {
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
		err := tcpProbe(addr)
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
			//log.Printf("host: %s\n", r.backend)
		} else {
			log.Printf("error: %s", r.err)
		}
	}
	//log.Printf("Active server: %v\n", backends)
	return backends, nil
}
