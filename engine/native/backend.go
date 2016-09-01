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
	logger "github.com/zhgwenming/gbalancer/log"
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
	weight  uint // as sequence in max heap, weight in min heap
	flags   BackendFlags

	failChan chan<- *spdySession
	tunnels  uint
	count    uint64
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string, tunnels uint, weight uint) *Backend {
	tunnel := make([]connTunnel, tunnels, tunnels)

	b := &Backend{
		tunnel:  tunnel,
		address: addr,
		weight:  weight,
		tunnels: tunnels,
		flags:   FlagInit,
	}
	logger.GlobalLog.Printf("Test_Issue: NewBackend is called successfully\n")

	return b
}

func (b *Backend) SwitchSpdyConn(index uint, to *connTunnel) {
	if from := b.tunnel[index].conn; from != nil {
		from.Close()
	}
	b.tunnel[index].conn = to.conn
	b.tunnel[index].tcpAddr = to.tcpAddr
	b.tunnel[index].switching = false
	logger.GlobalLog.Printf("Test_Issue: SwitchSpdyConn is called successfully\n")
}

func (b *Backend) FailChan(fail chan<- *spdySession) {
	b.failChan = fail
}

// Create new tunnel session if the streamId almost used up
func (b *Backend) SpdyCheckStreamId(backChan chan<- *spdySession) {

	// for whatever cases
	// increase the count number first
	b.count++

	if b.tunnels == 0 || time.Since(spdyCheckTime) < 5*time.Second {
		logger.GlobalLog.Printf("Test_Issue: Tunnels num is %d\n", b.tunnels)
		logger.GlobalLog.Printf("Test_Issue: SpdyCheckStreamId end and not executing CreateSpdySession func\n")
		return
	}

	spdyCheckTime = time.Now()
	for index := uint(0); index < b.tunnels; index++ {
		tunnel := b.tunnel

		if tunnel[index].conn != nil {
			// pre-create spdyconn to avoid out of StreamId
			if !tunnel[index].switching {
				// check to see if the spdyConn needed to be switched
				if uint32(tunnel[index].conn.PeekNextStreamId()) > ThreshStreamId {
					logger.GlobalLog.Printf("pre-create new session for %s", b.address)
					tunnel[index].switching = true
		            logger.GlobalLog.Printf("Test_Issue: SpdyCheckStreamId execution NewSpdySession\n")
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
		logger.GlobalLog.Printf("Test_Issue: ForwarderNewConnection execution failure: Tunnels number<=0\n")
		return net.Dial("tcp", req.backend.address)
	}

    logger.GlobalLog.Printf("Test_Issue: ForwarderNewConnection execution successfully: Tunnels number>0\n")

	var found bool
	var conn net.Conn
	err := fmt.Errorf("No stream sesssion exist")

	cnt := uint(b.count)
	for i := uint(0); i < b.tunnels; i++ {

		index := (cnt + i) % b.tunnels
		spdyconn := b.tunnel[index].conn

		if spdyconn != nil {
			found = true
			conn, err = spdyconn.CreateStream(http.Header{}, nil, false)
			if err != nil {
				spdyptr := (*unsafe.Pointer)(unsafe.Pointer(&b.tunnel[index].conn))

				swapped := atomic.CompareAndSwapPointer(spdyptr, unsafe.Pointer(spdyconn), nil)
				if swapped {
					if conn == nil {
						// streamId used up
						logger.GlobalLog.Printf("Used up streamdID. (%s)", err)
					} else {
						logger.GlobalLog.Printf("Failed to create stream. (%s)", err)
					}

					// try to close exist session
					spdyconn.Close()
		            logger.GlobalLog.Printf("Test_Issue: ForwarderNewConnection execution NewSpdySession\n")
					b.failChan <- NewSpdySession(b, index)
				}
			} else {
				logger.GlobalLog.Printf("Test_Issue: spdyconn.CreateStream is called successfully\n")
				break
			}
		}
	}

	if err != nil {
		// just log error if we have at lease one connection in the tunnel
		// if we don't, just fall back to tcp mode silently
		if found {
			logger.GlobalLog.Printf("Failed to create stream, rolling back to tcp mode. (%s)", err)
		}
		conn, err = net.Dial("tcp", req.backend.address)
	}

	return conn, err

}
