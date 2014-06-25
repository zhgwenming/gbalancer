// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package config

import (
	"fmt"
	"net"
	"strings"
)

type ListenAddr struct {
	net   string
	laddr string
}

func (l *ListenAddr) Listen() (net.Listener, error) {
	return net.Listen(l.net, l.laddr)
}

type Configuration struct {
	Service    string
	ExtCommand string
	User       string
	Pass       string
	Addr       string
	Port       string
	Listen     []string
	Backend    []string
}

func (c *Configuration) ListenInfo() string {
	return fmt.Sprintf("Listen on %v, backend: %v", c.Listen, c.Backend)
}

func (c *Configuration) AddListen(listen string) {
	c.Listen = append(c.Listen, listen)
}

func (c *Configuration) GetListenAddrs() ([]ListenAddr, error) {
	laddrs := make([]ListenAddr, 0, len(c.Listen))
	for _, l := range c.Listen {
		protoAddrParts := strings.SplitN(l, "://", 2)
		if len(protoAddrParts) != 2 {
			err := fmt.Errorf("incorrect listen addr %s", l)
			return laddrs, err
		}
		addr := ListenAddr{protoAddrParts[0], protoAddrParts[1]}
		laddrs = append(laddrs, addr)
	}

	return laddrs, nil
}
