// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
)

type BackendFlags uint16

const (
	FlagInit BackendFlags = 0x1
)

type Backend struct {
	spdyconn *spdystream.Connection
	address  string
	flags    BackendFlags
	index    int
	ongoing  uint
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string, useTunnel bool) *Backend {
	b := &Backend{address: addr, flags: FlagInit}
	if useTunnel {
		// asynchronous create a spdy connection
		go func() {
			//addrs := strings.Split(addr, ":")
			if conn, err := NewStreamConn("127.0.0.1", STREAMPORT); err == nil {
				b.spdyconn = conn
			}
		}()
	}
	return b
}
