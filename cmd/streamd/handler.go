// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/zhgwenming/gbalancer/Godeps/_workspace/src/github.com/docker/spdystream"
	"io"
	"net"
	"net/http"
)

type copyRet struct {
	bytes int64
	err   error
}

func streamCopy(dst io.WriteCloser, src io.Reader) {
	io.Copy(dst, src)
	dst.Close()
}

// Tunnel Handler
func AgentStreamHandler(stream *spdystream.Stream) {
	conn, err := net.Dial("unix", *serviceAddr)
	//conn, err := net.Dial("tcp", "10.100.91.74:3306")
	if err != nil {
		log.Printf("Failed: %s\n", err)
		return
	}

	replyErr := stream.SendReply(http.Header{}, false)
	if replyErr != nil {
		return
	}

	// drain the header requests to avoid DoS
	go func() {
		for {
			stream.ReceiveHeader()
		}
	}()

	go streamCopy(stream, conn)
	go streamCopy(conn, stream)

}
