// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"errors"
	"flag"
	"github.com/coreos/go-etcd/etcd"
	"github.com/zhgwenming/gbalancer/utils"
	"log"
	"os"
	"path"
	"strconv"
	"syscall"
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
//     │        │     ├── leader ── id {pid}
//     │        │     ├── resource
//     │        │     │    ├── createdIndex
//     │        │     │    ├── ...
//     │        │     │    └── createdIndexN
//     │        │     ├── node
//     │        │     │    ├── node1 {ip}
//     │        │     │    ├── node2
//     │        │     │    ├── ...
//     │        │     │    └── nodeN
//     │        │     └── config
//     │        │
//     │        ├── clusterN
//     │
//     ├── serviceNameN

type Ldirector struct {
	ClusterName string
	etcdClient  *etcd.Client
	IPAddress   string
	Pid         string
}

func NewLdirector(name string, etc *etcd.Client) *Ldirector {
	ip := utils.GetFirstIPAddr()
	pid := strconv.Itoa(os.Getpid())
	return &Ldirector{name, etc, ip, pid}
}

func (l Ldirector) Prefix() string {
	return path.Join(ServiceName, l.ClusterName)
}

func (l Ldirector) LeaderPath() string {
	return path.Join(l.Prefix(), "leader", "id")
}

func (l Ldirector) NodePath() string {
	return path.Join(l.Prefix(), "node", l.IPAddress)
}

// Node Register, dead instance on same node should be replaced ASAP
// to avoid service redistribution
// Issue exist if multiple instances share the same netns but not processns
// so this process shouldn't run inside containers
func (l *Ldirector) Register(ttl uint64) error {
	client := l.etcdClient
	pid := l.Pid

	nodePath := l.NodePath()

	resp, err := client.Get(nodePath, false, false)
	if err != nil {
		log.Printf("%s", err)
		_, err = client.Create(nodePath, pid, ttl)

		return err
	} else {
		// found a exist node
		value := resp.Node.Value
		existPid, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		log.Printf("Found a instance on this node: %d", existPid)

		// find the process
		// err always is nil on unix systems, so ignore it
		process, _ := os.FindProcess(existPid)

		err = process.Signal(syscall.Signal(0))
		//fmt.Printf("process.Signal on pid %d returned: %v\n", existPid, err)

		if err == nil {
			err = errors.New("A instance exist on this node")
			return err
		} else {
			// process doesn't exist on this machine, update the node to this instance
			log.Printf("Instance dead, replacing node %s to pid %s", nodePath, pid)
			_, err = client.Update(nodePath, pid, ttl)
			return err
		}
	}

}

// leader election can take some time to wait ttl expires
func (l *Ldirector) BecomeLeader(ttl uint64) {
	client := l.etcdClient
	id := l.IPAddress
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

	client, err := etcd.NewClientFromFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	director := NewLdirector(*clusterName, client)
	log.Printf("Starting with ip: %s", director.IPAddress)
	if err = director.Register(ttl); err != nil {
		log.Fatal(err)
	}

	director.BecomeLeader(ttl)

}
