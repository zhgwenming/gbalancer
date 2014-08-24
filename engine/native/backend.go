// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"fmt"
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
	tunnel  []connTunnel
	address string
	index   int // heap related fields
	ongoing uint
	flags   BackendFlags

	failChan *chan *spdySession
	tunnels  int
	count    uint64
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string, tunnels int) *Backend {
	tunnel := make([]connTunnel, tunnels, tunnels)

	b := &Backend{
		tunnel:  tunnel,
		address: addr,
		tunnels: tunnels,
		flags:   FlagInit,
	}

	return b
}

func (b *Backend) SwitchSpdyConn(index int, to *connTunnel) {
	if from := b.tunnel[index].conn; from != nil {
		from.Close()
	}
	b.tunnel[index].conn = to.conn
	b.tunnel[index].tcpAddr = to.tcpAddr
	b.tunnel[index].switching = false
}

// Create new tunnel session if the streamId almost used up
func (b *Backend) SpdyCheckStreamId(backChan chan<- *spdySession) {

	// for whatever cases
	// increase the count number first
	b.count++

	if b.tunnels == 0 || time.Since(spdyCheckTime) < 5*time.Second {
		return
	}

	spdyCheckTime = time.Now()
	for index := 0; index < b.tunnels; index++ {
		tunnel := b.tunnel

		if tunnel[index].conn != nil {
			// pre-create spdyconn to avoid out of StreamId
			if !tunnel[index].switching {
				tunnel[index].switching = true
				// check to see if the spdyConn needed to be switched
				if uint32(tunnel[index].conn.PeekNextStreamId()) > ThreshStreamId {
					log.Printf("pre-create new session for %s", b.address)
					go CreateSpdySession(NewSpdySession(b, index), backChan)
				}
			}
		}
	}
}

// Runs inside of Forwarder goroutine
// takeoff the spdyconn if it's broken
func (b *Backend) ForwarderNewConnection(req *Request) (net.Conn, error) {
	if b.tunnels <= 0 {
		return net.Dial("tcp", req.backend.address)
	}

	var conn net.Conn
	err := fmt.Errorf("No stream sesssion exist")

	cnt := int(b.count)
	for i := 0; i < b.tunnels; i++ {

		index := (cnt + i) % b.tunnels
		spdyconn := b.tunnel[index].conn

		if spdyconn != nil {
			conn, err = spdyconn.CreateStream(http.Header{}, nil, false)
			if err != nil {
				spdyptr := (*unsafe.Pointer)(unsafe.Pointer(&b.tunnel[index].conn))

				swapped := atomic.CompareAndSwapPointer(spdyptr, unsafe.Pointer(spdyconn), nil)
				if swapped {
					if conn == nil {
						// streamId used up
						log.Printf("Used up streamdID. (%s)", err)
					} else {
						log.Printf("Failed to create stream. (%s)", err)
					}

					// try to close exist session
					spdyconn.Close()
					*b.failChan <- NewSpdySession(b, index)
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
