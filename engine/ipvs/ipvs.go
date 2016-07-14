// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package ipvs

import (
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/utils"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	IPvsLocalAddr = "127.1.1.1"
)

type IPvs struct {
	Addr      string
	Port      string
	Scheduler string
	done      <-chan struct{}
	WGroup    *sync.WaitGroup
	backends  map[string]string
	Persist   int
}

func NewIPvs(addr, port, sch string, done <-chan struct{}, wgroup *sync.WaitGroup) *IPvs {
	backends := make(map[string]string, 4)
	return &IPvs{addr, port, sch, done, wgroup, backends, 300}
}

func runCommand(cmd string) error {
	args := strings.Fields(cmd)
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Err: %s Output: %s, Cmd %s", err, output, cmd)
		logger.GlobalLog.Printf("%s", err)
	}
	return err
}

func ensureCommands(cmds []string) error {
	for _, c := range cmds {
		if err := runCommand(c); err != nil {
			return err
		}
	}
	return nil
}

func (i *IPvs) eventLoop(status <-chan map[string]int) {
	for {
		select {
		case backends := <-status:
			if len(backends) == 0 {
				logger.GlobalLog.Printf("balancer: got empty backends list")
			}

			for addr, _ := range i.backends {
				if _, ok := backends[addr]; !ok {
					i.RemoveBackend(addr)
				} else {
					delete(backends, addr)
				}
			}

			// the rest of active backends, add them
			for addr, _ := range backends {
				i.AddBackend(addr)
			}
		case <-i.done:
			return
		}
	}
}

//# Source NAT for VIP 192.168.100.30:80
//% iptables -t nat -A POSTROUTING -m ipvs --vaddr 192.168.100.30/32 \
//> --vport 80 -j SNAT --to-source 192.168.10.10
//
//or SNAT-ing only a specific real server:
//
//% iptables -t nat -A POSTROUTING --dst 192.168.11.20 \
//> -m ipvs --vaddr 192.168.100.30/32 -j SNAT --to-source 192.168.10.10

// routing table
//% ip route  add  table local 127.1.1.1 dev lo  proto kernel  scope host  src 172.16.154.164
//% ip route flush cache
func AddLocalRoute(dst, src string) {
	cmd := "ip route add table local " + dst + " dev lo proto kernel scope host src " + src
	runCommand(cmd)

	// flush the routing cache
	cmd = "ip route flush cache"
	runCommand(cmd)

}

func DeleteLocalRoute(dst string) {
	cmd := "ip route delete table local " + dst
	runCommand(cmd)
	cmd = "ip route flush cache"
	runCommand(cmd)
}

func (i *IPvs) LocalSchedule(status <-chan map[string]int) {
	var cmd string
	if output, err := exec.Command("ipvsadm", "-A",
		"-t", i.Addr+":"+i.Port,
		"-s", i.Scheduler).CombinedOutput(); err != nil {
		logger.GlobalLog.Printf("ipvs init: %s", err)
		logger.GlobalLog.Printf("ipvs: %s", output)
		os.Exit(1)
	}
	defer func() {
		cmd = "ipvsadm -D -t " + i.Addr + ":" + i.Port
		runCommand(cmd)
		i.WGroup.Done()
	}()

	useAddr := utils.GetFirstIPAddr()

	AddLocalRoute(i.Addr, useAddr)

	// to enable multiple instances of gbalancer exist, just keep the route
	//defer DeleteLocalRoute(i.Addr)

	i.eventLoop(status)
}

func (i *IPvs) RemoteSchedule(status <-chan map[string]int) {
	var cmd string
	cmds := []string{
		"sysctl -w net.ipv4.ip_forward=1",
		"sysctl -w net.ipv4.vs.conntrack=1",
	}

	if err := ensureCommands(cmds); err != nil {
		logger.GlobalLog.Fatal(err)
	}

	if output, err := exec.Command("ipvsadm", "-A",
		"-t", i.Addr+":"+i.Port,
		"-s", i.Scheduler,
		"-p", strconv.Itoa(i.Persist)).CombinedOutput(); err != nil {
		err = fmt.Errorf("Init Err: %s Output: %s", err, output)
		logger.GlobalLog.Fatal(err)
	}
	defer func() {
		cmd = "ipvsadm -D -t " + i.Addr + ":" + i.Port
		runCommand(cmd)
		i.WGroup.Done()
	}()

	localAddr := utils.GetFirstIPAddr()
	// % iptables -t nat -A POSTROUTING -m ipvs --vaddr 192.168.100.30/32 --vport 80 -j SNAT --to-source 192.168.10.10
	rule := "POSTROUTING -m ipvs --vaddr " + i.Addr + "/32 --vport " + i.Port + " -j SNAT --to " + localAddr
	cmd = "iptables -t nat -A " + rule
	runCommand(cmd)
	defer func() {
		cmd = "iptables -t nat -D " + rule
		runCommand(cmd)
	}()

	i.eventLoop(status)
}
func (i *IPvs) AddBackend(addr string) {
	logger.GlobalLog.Printf("balancer: bring up %s.\n", addr)
	srv := i.Addr + ":" + i.Port
	if output, err := exec.Command("ipvsadm", "-a",
		"-t", srv,
		"-r", addr, "-m").CombinedOutput(); err != nil {
		err = fmt.Errorf("Add Err: %s Output: %s, Addr %s", err, output, addr)
		logger.GlobalLog.Printf("%s", err)
	}

	i.backends[addr] = addr
}

func (i *IPvs) RemoveBackend(addr string) {
	logger.GlobalLog.Printf("balancer: take down %s.\n", addr)
	srv := i.Addr + ":" + i.Port
	if _, ok := i.backends[addr]; ok {
		if output, err := exec.Command("ipvsadm", "-d",
			"-t", srv,
			"-r", addr).CombinedOutput(); err != nil {
			err = fmt.Errorf("Remove Err: %s Output: %s", err, output)
			logger.GlobalLog.Printf("%s", err)
		}
		delete(i.backends, addr)
	} else {
		logger.GlobalLog.Printf("balancer: %s is not up, bug might exist!", addr)
	}
}
