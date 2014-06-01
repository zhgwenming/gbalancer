// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/coreos/go-etcd/etcd"
	"log"
)

func main() {
	cluster := "ldirector"

	server := []string{
		"http://127.0.0.1:4001",
	}

	cl := etcd.NewClient(server)

	// curl -X PUT http://127.0.0.1:4001/mod/v2/leader/{clustername}?ttl=60 -d name=servername
	// not supported by etcd client yet
	// so we create a new key and ignore the return value first.
	_, err := cl.Create(cluster, cluster, 0)
	if err != nil {
		log.Fatal(err)
	}
}
