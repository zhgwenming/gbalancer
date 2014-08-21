// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import ()

type BackendFlags uint16

const (
	FlagInit BackendFlags = 0x1
)

type Backend struct {
	spdyconn *spdyConn
	address  string
	flags    BackendFlags
	index    int
	ongoing  uint
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string) *Backend {
	b := &Backend{address: addr, flags: FlagInit}
	return b
}

func (b *Backend) SwitchSpdyConn(to *spdyConn) {
	from := b.spdyconn
	from.conn.Close()
	b.spdyconn = to

}
