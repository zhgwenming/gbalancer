// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"net"
	"net/http"
	"sync/atomic"
	"unsafe"
)

type BackendFlags uint16

const (
	FlagInit BackendFlags = 0x1
)

type Backend struct {
	spdyconn []*spdyConn
	address  string
	tunnels  int
	flags    BackendFlags
	index    int
	count    uint64
	ongoing  uint
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string, tunnels int) *Backend {
	conn := make([]*spdyConn, tunnels, tunnels)

	b := &Backend{
		spdyconn: conn,
		address:  addr,
		tunnels:  tunnels,
		flags:    FlagInit,
	}

	return b
}

func (b *Backend) SwitchSpdyConn(index int, to *spdyConn) {
	from := b.spdyconn[index]
	from.conn.Close()
	b.spdyconn[index] = to
}

func (b *Backend) SpdyCheck(spdyChan chan<- *spdySession) {
	if b.tunnels > 0 {
		b.count++

		index := int(b.count) / b.tunnels
		spdyconn := b.spdyconn[index]

		if !spdyconn.switching {
			spdyconn.switching = true
			// check to see if the spdyConn needed to be switched
			if uint32(spdyconn.conn.PeekNextStreamId()) > ThreshStreamId {
				spdyChan <- NewSpdySession(b, index)
			}
		}
	}
}

// Runs inside of Forwarder goroutine
func (b *Backend) ForwarderNewConnection(req *Request) (net.Conn, error) {
	var conn net.Conn
	var err error

	cnt := int(b.count)

	for i := 0; i < b.tunnels; i++ {

		index := (cnt + i) / b.tunnels
		spdyconn := b.spdyconn[index]

		if spdyconn != nil {
			conn, err = spdyconn.conn.CreateStream(http.Header{}, nil, false)
			if err != nil {
				spdyptr := (*unsafe.Pointer)(unsafe.Pointer(&b.spdyconn[index]))

				swapped := atomic.CompareAndSwapPointer(spdyptr, unsafe.Pointer(spdyconn), nil)
				if swapped {
					if conn == nil {
						// streamId used up
						log.Printf("Used up streamdID. (%s)", err)
					} else {
						log.Printf("Failed to create stream. (%s)", err)
					}

					// try to close exist session
					spdyconn.conn.Close()
				}
			} else {
				break
			}
		}
	}

	if err != nil {
		log.Printf("Failed to create stream, roll back to tcp mode. (%s)", err)
		conn, err = net.Dial("tcp", req.backend.address)
	}

	return conn, err

}
