// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"net"
	//"strings"
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
	"net/http"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	STREAMPORT = "6900"
)

type spdyConn struct {
	conn      *spdystream.Connection
	tcpAddr   *net.TCPAddr
	switching bool
}

type spdySession struct {
	backend *Backend
	spdy    *spdyConn
	index   int
}

func NewSpdySession(backend *Backend, index int) *spdySession {
	return &spdySession{backend: backend, index: index}
}

func (spdy *spdyConn) CreateStream(headers http.Header, parent *spdystream.Stream, fin bool) (*spdystream.Stream, error) {
	conn, err := spdy.conn.CreateStream(http.Header{}, nil, false)

	// error to create a new stream
	if err != nil {
		//req.backend.spdyconn = nil
		tcpconn := spdy.conn
		spdyptr := (*unsafe.Pointer)(unsafe.Pointer(&spdy.conn))
		swapped := atomic.CompareAndSwapPointer(spdyptr, unsafe.Pointer(tcpconn), nil)
		if swapped {
			if conn == nil {
				// streamId used up
				// TODO:
				// create a new spdy connection
				spdy.conn.Close()
				log.Printf("Used up streamdID, roll back to tcp mode. (%s)", err)
			} else {
				log.Printf("Failed to create stream, roll back to tcp mode. (%s)", err)
			}
		}
	} else {
		// TODO: spdy conn pre-creation
	}
	return conn, err
}

func NewSpdyConn(conn net.Conn) *spdyConn {
	var spdyconn *spdyConn

	if conn == nil {
		return nil
	}

	addr := conn.LocalAddr()

	if tcpaddr, ok := addr.(*net.TCPAddr); !ok {
		return nil
	} else {
		spdy, err := spdystream.NewConnection(conn, false)
		if err != nil {
			log.Printf("spdystream create connection error: %s", err)
			return nil
		}

		go spdy.Serve(spdystream.NoOpStreamHandler)
		if _, err = spdy.Ping(); err != nil {
			return nil
		}

		spdyconn = &spdyConn{conn: spdy, tcpAddr: tcpaddr, switching: false}
	}

	return spdyconn
}

func NewStreamConn(addr, port string) (*spdyConn, error) {
	conn, err := net.DialTimeout("tcp", addr+":"+port, time.Second)
	if err != nil {
		log.Printf("dail spdy error: %s", err)
		return nil, err
	}

	spdyConn := NewSpdyConn(conn)

	return spdyConn, nil
}

func SpdySessionManager(request <-chan *spdySession, ready chan<- *spdySession) {
	for session := range request {

		log.Printf("Creating new session for: %s", session.backend.address)
		//addrs := strings.Split(backend.address, ":")
		if conn, err := NewStreamConn("127.0.0.1", STREAMPORT); err == nil {
			session.spdy = conn
		}

		ready <- session
	}
}
