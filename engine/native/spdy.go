// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
	"net"
	"strings"
	"time"
)

const (
	STREAMPORT = "6900"
)

type connTunnel struct {
	conn      *spdystream.Connection
	tcpAddr   *net.TCPAddr
	switching bool
}

type spdySession struct {
	backend   *Backend
	spdy      *connTunnel
	connindex uint
}

func NewSpdySession(backend *Backend, index uint) *spdySession {
	return &spdySession{backend: backend, connindex: index}
}

func NewConnTunnel(conn net.Conn) *connTunnel {
	var spdyconn *connTunnel

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

		spdyconn = &connTunnel{conn: spdy, tcpAddr: tcpaddr, switching: false}
	}

	return spdyconn
}

func NewStreamConn(addr, port string) (*connTunnel, error) {
	conn, err := net.DialTimeout("tcp", addr+":"+port, time.Second)
	if err != nil {
		//log.Printf("dail spdy error: %s", err)
		return nil, err
	}

	connTunnel := NewConnTunnel(conn)

	return connTunnel, nil
}

func CreateSpdySession(request *spdySession, ready chan<- *spdySession) {
	for {
		addrs := strings.Split(request.backend.address, ":")
		if conn, err := NewStreamConn(addrs[0], STREAMPORT); err == nil {
			request.spdy = conn
			log.Printf("Created new session for: %s", request.backend.address)
			break
		}
		time.Sleep(time.Second)
	}
	ready <- request
}
