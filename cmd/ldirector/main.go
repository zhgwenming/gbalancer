// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/zhgwenming/gbalancer/utils"
	"log"
	"os"
	"strconv"
	"time"
)

const (
	ttl = 60
)

func BecomeLeader(cl *etcd.Client, identity string, ttl uint64) {
	sleeptime := time.Duration(ttl / 3)
	//log.Printf("Sleep time is %d", sleeptime)

	cluster := "leader/ldirector"

	for {
		// curl -X PUT http://127.0.0.1:4001/mod/v2/leader/{clustername}?ttl=60 -d name=servername
		// not supported by etcd client yet
		// so we create a new key and ignore the return value first.
		if _, err := cl.Create(cluster, identity, ttl); err != nil {
			time.Sleep(5 * time.Second)
		} else {
			log.Printf("No leader exist, taking the leadership")
			go func() {
				for {
					time.Sleep(sleeptime * time.Second)
					// update the ttl periodically, should never get error
					_, err = cl.CompareAndSwap(cluster, identity, ttl, identity, 0)
					if err != nil {
						log.Fatal(err)
					}
				}
			}()
		}
	}
}

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

	BecomeLeader(cl, identity, ttl)

}
