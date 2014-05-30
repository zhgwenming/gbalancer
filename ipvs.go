// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

type IPvs struct {
	Addr      string
	Port      string
	Scheduler string
	backends  map[string]string
	Persist   int
}

func NewIPvs(addr, port, sch string) *IPvs {
	backends := make(map[string]string, 4)
	return &IPvs{addr, port, sch, backends, 300}
}

func runCommand(cmd string) error {
	args := strings.Split(cmd, " ")
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Err: %s Output: %s, Cmd %s", err, output, cmd)
		slog.Printf("%s", err)
	}
	return err
}

func getIPAddr() (addr string) {
	addrs, _ := net.InterfaceAddrs()
	for _, i := range addrs {
		ipnet, ok := i.(*net.IPNet)

		if !ok {
			slog.Fatal("assertion err: %v\n", ipnet)
		}

		ip4 := ipnet.IP.To4()

		if !ip4.IsLoopback() {
			addr = ip4.String()
			break
		}
	}
	slog.Printf("%v", addr)
	return
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
func (i *IPvs) schedule(status <-chan map[string]int) {
	var cmd string
	if output, err := exec.Command("ipvsadm", "-A",
		"-t", IPvsAddr+":"+i.Port,
		"-s", i.Scheduler,
		"-p", strconv.Itoa(i.Persist)).CombinedOutput(); err != nil {
		err = fmt.Errorf("Init Err: %s Output: %s", err, output)
		slog.Fatal(err)
	}
	defer func() {
		cmd = "ipvsadm -D -t " + IPvsAddr + ":" + i.Port
		runCommand(cmd)
	}()

	localAddr := getIPAddr()
	// % iptables -t nat -A POSTROUTING -m ipvs --vaddr 192.168.100.30/32 --vport 80 -j SNAT --to-source 192.168.10.10
	cmd = "iptables -t nat -A POSTROUTING -m ipvs --vaddr " + IPvsAddr + " --vport " + i.Port + " -j SNAT --to " + localAddr
	runCommand(cmd)

	cmd = "ip route add table local " + IPvsAddr + " dev lo proto kernel scope host src " + localAddr
	runCommand(cmd)
	defer func() {
		cmd = "ip route  delete  table local " + IPvsAddr
		runCommand(cmd)
		cmd = "ip route flush cache"
		runCommand(cmd)
	}()

	cmd = "ip route flush cache"
	runCommand(cmd)

	for {
		select {
		case backends := <-status:
			if len(backends) == 0 {
				slog.Printf("balancer: got empty backends list")
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
		}
	}
}

func (i *IPvs) AddBackend(addr string) {
	slog.Printf("balancer: bring up %s.\n", addr)
	srv := IPvsAddr + ":" + i.Port
	if output, err := exec.Command("ipvsadm", "-a",
		"-t", srv,
		"-r", addr, "-m").CombinedOutput(); err != nil {
		err = fmt.Errorf("Add Err: %s Output: %s, Addr %s", err, output, addr)
		slog.Printf("%s", err)
	}

	i.backends[addr] = addr
}

func (i *IPvs) RemoveBackend(addr string) {
	slog.Printf("balancer: take down %s.\n", addr)
	srv := i.Addr + ":" + i.Port
	if _, ok := i.backends[addr]; ok {
		if output, err := exec.Command("ipvsadm", "-d",
			"-t", srv,
			"-r", addr).CombinedOutput(); err != nil {
			err = fmt.Errorf("Remove Err: %s Output: %s", err, output)
			slog.Printf("%s", err)
		}
		delete(i.backends, addr)
	} else {
		slog.Printf("balancer: %s is not up, bug might exist!", addr)
	}
}
