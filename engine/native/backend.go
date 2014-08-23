// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"net"
	"net/http"
	"sync/atomic"
	"time"
	"unsafe"
)

type BackendFlags uint16

const (
	FlagInit BackendFlags = 0x1
)

var (
	spdyCheckTime time.Time
)

type Backend struct {
	spdyconn   []*spdyConn
	address    string
	index      int // heap related fields
	ongoing    uint
	flags      BackendFlags
	tunnelChan *chan *spdySession // tunnel related fields
	tunnels    int
	count      uint64
	RxBytes    uint64
	TxBytes    uint64
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

// Create new tunnel session if necessary
func (b *Backend) SpdyCheck() {
	if b.tunnels > 0 && time.Since(spdyCheckTime) > 5*time.Second {
		spdyCheckTime = time.Now()
		for index := 0; index < b.tunnels; index++ {
			spdyconn := b.spdyconn[index]

			if spdyconn != nil {
				// pre-create spdyconn to avoid out of StreamId
				if !spdyconn.switching {
					spdyconn.switching = true
					// check to see if the spdyConn needed to be switched
					if uint32(spdyconn.conn.PeekNextStreamId()) > ThreshStreamId {
						log.Printf("pre-create new session for %s", b.address)
						go CreateSpdySession(NewSpdySession(b, index), *b.tunnelChan)
					}
				}
			} else {
				log.Printf("create new session for %s", b.address)
				go CreateSpdySession(NewSpdySession(b, index), *b.tunnelChan)
			}
		}
	}
}

// Runs inside of Forwarder goroutine
// takeoff the spdyconn if it's broken
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
