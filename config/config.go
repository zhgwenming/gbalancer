// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package config

import (
	"fmt"
	"strings"
)

type ListenAddr struct {
	net   string
	laddr string
}

type Configuration struct {
	Service    string
	ExtCommand string
	User       string
	Pass       string
	Addr       string
	Port       string
	UnixSocket string
	Listen     []string
	Backend    []string
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
