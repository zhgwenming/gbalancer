// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"flag"
	"fmt"
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

func (l *Ldirector) FindInstance() (int, error) {
	var pid int
	client := l.etcdClient
	nodePath := l.NodePath()

	resp, err := client.Get(nodePath, false, false)
	if err != nil {
		//log.Printf("No node defined in etcd")
		return pid, err
	} else {
		// found a exist node
		value := resp.Node.Value
		pid, err = strconv.Atoi(value)
		if err != nil {
			log.Printf("Got a wrong format of pid %v", value)
			return pid, err
		} else {
			// err of os.FindProcess() is always nil in unix system
			process, _ := os.FindProcess(pid)
			err = process.Signal(syscall.Signal(0))
			return pid, err
		}
	}
}

// Node Register, dead instance on same node should be replaced ASAP
// to avoid service redistribution
func (l *Ldirector) Register(ttl uint64) error {
	client := l.etcdClient
	pid := l.Pid
	nodePath := l.NodePath()

	for {
		existPid, err := l.FindInstance()
		//log.Printf("error is %s", err)
		// found a running pid
		if err == nil {
			err = fmt.Errorf("A exist instance on this node running with %d", existPid)
			return err
		}

		// Issue will exist if we use Update here
		// since multiple instances share the same netns but not processns will make multiple instances runable
		// so we always use Create()
		if _, err := client.Create(nodePath, pid, ttl); err != nil {
			// to avoid the following race conditions:
			// 1. multiple instances might be waiting on this node
			// 2. a instance on same node shares same netns but different processns, will
			// 	cause this looping forever
			log.Printf("No instance exist on this node, waiting ttl to expire")
			time.Sleep(time.Second)
		} else {
			log.Printf("No instance exist on this node, starting")
			go func() {
				sleeptime := time.Duration(ttl / 3)
				for {
					time.Sleep(sleeptime * time.Second)
					// update the ttl periodically, should never get error
					_, err = client.CompareAndSwap(nodePath, pid, ttl, pid, 0)
					if err != nil {
						log.Fatal(err)
					}
				}
			}()
			return nil
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
	log.Printf("Starting with node: %s", director.NodePath())
	if err = director.Register(ttl); err != nil {
		log.Fatal(err)
	}

	director.BecomeLeader(ttl)

}
