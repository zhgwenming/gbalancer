// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"net"
	//"strings"
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
	"time"
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
	backend   *Backend
	spdy      *spdyConn
	connindex int
}

func NewSpdySession(backend *Backend, index int) *spdySession {
	return &spdySession{backend: backend, connindex: index}
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

func CreateSpdySession(request *spdySession, ready chan<- *spdySession) {
	for {
		//addrs := strings.Split(backend.address, ":")
		if conn, err := NewStreamConn("127.0.0.1", STREAMPORT); err == nil {
			request.spdy = conn
			log.Printf("Created new session for: %s", request.backend.address)
			break
		}
		time.Sleep(time.Second)
	}
	ready <- request
}
