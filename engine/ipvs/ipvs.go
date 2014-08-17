// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package ipvs

import (
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"net"
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

var (
	log = logger.NewLogger()
)

func NewIPvs(addr, port, sch string, done <-chan struct{}, wgroup *sync.WaitGroup) *IPvs {
	backends := make(map[string]string, 4)
	return &IPvs{addr, port, sch, done, wgroup, backends, 300}
}

func runCommand(cmd string) error {
	args := strings.Fields(cmd)
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Err: %s Output: %s, Cmd %s", err, output, cmd)
		log.Printf("%s", err)
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

//func getIPAddr() (addr string) {
//	addrs, _ := net.InterfaceAddrs()
//	for _, i := range addrs {
//		ipnet, ok := i.(*net.IPNet)
//
//		if !ok {
//			log.Fatal("assertion err: %v\n", ipnet)
//		}
//
//		ip4 := ipnet.IP.To4()
//
//		if !ip4.IsLoopback() {
//			addr = ip4.String()
//			break
//		}
//	}
//	log.Printf("%v", addr)
//	return
//}
//
func getIPAddr() (addr string) {
	ifaces, _ := net.Interfaces()

iface:
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}

		if addrs, err := i.Addrs(); err != nil {
			continue
		} else {
			for _, ipaddr := range addrs {
				//log.Printf("%v", ipaddr)
				ipnet, ok := ipaddr.(*net.IPNet)

				if !ok {
					log.Fatal("assertion err: %v\n", ipnet)
				}

				ip4 := ipnet.IP.To4()
				if ip4 == nil {
					continue
				}
				//log.Printf("%v", ip4)

				if !ip4.IsLoopback() {
					addr = ip4.String()
					break iface
				}
			}
		}
	}
	log.Printf("Found local ip4 %v", addr)
	return
}
func (i *IPvs) eventLoop(status <-chan map[string]int) {
	for {
		select {
		case backends := <-status:
			if len(backends) == 0 {
				log.Printf("balancer: got empty backends list")
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
		log.Printf("ipvs init: %s", err)
		log.Printf("ipvs: %s", output)
		os.Exit(1)
	}
	defer func() {
		cmd = "ipvsadm -D -t " + i.Addr + ":" + i.Port
		runCommand(cmd)
		i.WGroup.Done()
	}()

	useAddr := getIPAddr()

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
		log.Fatal(err)
	}

	if output, err := exec.Command("ipvsadm", "-A",
		"-t", i.Addr+":"+i.Port,
		"-s", i.Scheduler,
		"-p", strconv.Itoa(i.Persist)).CombinedOutput(); err != nil {
		err = fmt.Errorf("Init Err: %s Output: %s", err, output)
		log.Fatal(err)
	}
	defer func() {
		cmd = "ipvsadm -D -t " + i.Addr + ":" + i.Port
		runCommand(cmd)
		i.WGroup.Done()
	}()

	localAddr := getIPAddr()
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
	log.Printf("balancer: bring up %s.\n", addr)
	srv := i.Addr + ":" + i.Port
	if output, err := exec.Command("ipvsadm", "-a",
		"-t", srv,
		"-r", addr, "-m").CombinedOutput(); err != nil {
		err = fmt.Errorf("Add Err: %s Output: %s, Addr %s", err, output, addr)
		log.Printf("%s", err)
	}

	i.backends[addr] = addr
}

func (i *IPvs) RemoveBackend(addr string) {
	log.Printf("balancer: take down %s.\n", addr)
	srv := i.Addr + ":" + i.Port
	if _, ok := i.backends[addr]; ok {
		if output, err := exec.Command("ipvsadm", "-d",
			"-t", srv,
			"-r", addr).CombinedOutput(); err != nil {
			err = fmt.Errorf("Remove Err: %s Output: %s", err, output)
			log.Printf("%s", err)
		}
		delete(i.backends, addr)
	} else {
		log.Printf("balancer: %s is not up, bug might exist!", addr)
	}
}
