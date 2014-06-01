// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"github.com/coreos/go-etcd/etcd"
	"github.com/zhgwenming/gbalancer/utils"
	"log"
	"os"
	"path"
	"strconv"
	"time"
)

const (
	ServiceName = "ldirector"
	ttl         = 60
)

// etcd directory hierarchy:
// v2/keys
//     ├── serviceName
//     │        ├── cluster1
//     │        │     ├── leader ── id
//     │        │     ├── resource
//     │        │     ├── node
//     │        │     │    ├── node1
//     │        │     │    ├── node2
//     │        │     │    └── nodeN
//     │        │     └── config
//     │        │
//     │        ├── clusterN
//     │
//     ├── serviceNameN

type Ldirector struct {
	ClusterName string
	Identity    string
	etcdClient  *etcd.Client
}

func NewLdirector(name, id string, etc *etcd.Client) *Ldirector {
	return &Ldirector{name, id, etc}
}

func (l Ldirector) Prefix() string {
	return path.Join(ServiceName, l.ClusterName)
}

func (l Ldirector) LeaderPath() string {
	return path.Join(l.Prefix(), "leader", "id")

}
func (l *Ldirector) BecomeLeader(ttl uint64) {
	client := l.etcdClient
	id := l.Identity
	sleeptime := time.Duration(ttl / 3)
	//log.Printf("Sleep time is %d", sleeptime)

	leaderPath := l.LeaderPath()
	log.Printf("leader path: %s", leaderPath)

	for {
		// curl -X PUT http://127.0.0.1:4001/mod/v2/leader/{clustername}?ttl=60 -d name=servername
		// not supported by etcd client yet
		// so we create a new key and ignore the return value first.
		if _, err := client.Create(leaderPath, id, ttl); err != nil {
			time.Sleep(5 * time.Second)
		} else {
			log.Printf("No leader exist, taking the leadership")
			go func() {
				for {
					time.Sleep(sleeptime * time.Second)
					// update the ttl periodically, should never get error
					_, err = client.CompareAndSwap(leaderPath, id, ttl, id, 0)
					if err != nil {
						log.Fatal(err)
					}
				}
			}()
		}
	}
}

var (
	clusterName = flag.String("cluster", "clusterService1", "Cluster name")
)

func main() {

	//server := []string{
	//	"http://127.0.0.1:4001",
	//}
	//cl := etcd.NewClient(server)

	identity := utils.GetFirstIPAddr()
	identity += "_" + strconv.Itoa(os.Getpid())
	log.Printf("Starting with identity: %s", identity)

	cl, err := etcd.NewClientFromFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	director := NewLdirector(*clusterName, identity, cl)
	director.BecomeLeader(ttl)

}
