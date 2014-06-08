// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package cluster

import (
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

// etcd directory hierarchy:
// v2/keys
//     ├── serviceName
//     │        ├── cluster1
//     │        │     ├── leader ── id {ip}
//     │        │     ├── resource
//     │        │     │    ├── createdIndex
//     │        │     │    ├── ...
//     │        │     │    └── createdIndexN
//     │        │     ├── node
//     │        │     │    ├── (node1) ip1 {pid}
//     │        │     │    ├── (node2) ip2
//     │        │     │    ├── ...
//     │        │     │    └── (nodeN) ipN
//     │        │     └── config
//     │        │
//     │        ├── clusterN
//     │
//     ├── serviceNameN

type Client struct {
	ServiceName string
	ClusterName string
	etcdClient  *etcd.Client
	IPAddress   string
	Pid         string
}

func NewClient(service, cluster string, server []string) *Client {

	client := etcd.NewClient(server)

	ip := utils.GetFirstIPAddr()
	pid := strconv.Itoa(os.Getpid())
	return &Client{service, cluster, client, ip, pid}
}

func NewClientFromFile(service, cluster, config string) *Client {
	client, err := etcd.NewClientFromFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	ip := utils.GetFirstIPAddr()
	pid := strconv.Itoa(os.Getpid())
	return &Client{service, cluster, client, ip, pid}
}

func (l Client) Prefix() string {
	return path.Join(l.ServiceName, l.ClusterName)
}

func (l Client) LeaderPath() string {
	return path.Join(l.Prefix(), "leader", "id")
}

func (l Client) NodePath() string {
	return path.Join(l.Prefix(), "node", l.IPAddress)
}

func (l *Client) FindInstance() (int, error) {
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

func (l *Client) Lock(key, value string, ttl uint64) error {
	client := l.etcdClient

	// curl -X PUT http://127.0.0.1:4001/mod/v2/leader/{clustername}?ttl=60 -d name=servername
	// not supported by etcd client yet.
	// Issue will exist if we use Update here
	// since multiple instances share the same netns but not processns will make multiple instances runable
	// so we always use Create()
	if resp, err := client.Create(key, value, ttl); err != nil {
		// to avoid the following race conditions:
		// 1. multiple instances might be waiting on this node
		// 2. a instance on same node shares same netns but different processns, will
		// 	cause this looping forever
		log.Printf("Error to create node: %s", err)
		return err
	} else {
		//log.Printf("No instance exist on this node, starting")
		go func() {
			sleeptime := time.Duration(ttl / 3)
			for {
				index := resp.EtcdIndex
				time.Sleep(sleeptime * time.Second)
				// update the ttl periodically
				// error might happens for the following cases:
				// 1. node got modified unexpectedly
				// 2. we got stopped, process stucked
				resp, err = client.CompareAndSwap(key, value, ttl, value, index)
				if err != nil {
					log.Fatal("Unexpected lost our node lock", err)
				}
			}
		}()
		return nil
	}
}

// Node Register, dead instance on same node should be
// replaced ASAP to avoid service redistribution
func (l *Client) Register(ttl uint64) error {
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

		if err := l.Lock(nodePath, pid, ttl); err == nil {
			log.Printf("No instance exist on this node, starting")
			return nil
		}

		// retry after 1s
		time.Sleep(time.Second)
	}
}

// leader election can take some time to wait ttl expires
func (l *Client) BecomeLeader(ttl uint64) {
	client := l.etcdClient
	id := l.IPAddress
	sleeptime := time.Duration(ttl / 3)
	//log.Printf("Sleep time is %d", sleeptime)

	leaderPath := l.LeaderPath()
	log.Printf("leader path: %s", leaderPath)

	for {
		if err := l.Lock(leaderPath, id, ttl); err == nil {
			log.Printf("No leader exist, taking the leadership")
			return
		}

		// retry after 5 secs
		time.Sleep(5 * time.Second)
	}
}
