// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"github.com/zhgwenming/gbalancer/cluster"
	"log"
	"time"
)

const (
	ServiceName = "ldirector"
	ttl         = 60
)

var (
	clusterName = flag.String("cluster", "clusterService1", "Cluster name")
)

func main() {
	director := cluster.NewClientFromFile(ServiceName, *clusterName, "config.json")

	log.Printf("Starting with node: %s", director.NodePath())
	if err := director.Register(ttl); err != nil {
		log.Fatal(err)
	}

	director.BecomeLeader(ttl)
	for {
		time.Sleep(time.Second)
	}
}
