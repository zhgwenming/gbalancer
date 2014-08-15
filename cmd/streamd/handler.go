// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/docker/spdystream"
	"io"
	"net"
	"net/http"
)

type copyRet struct {
	bytes int64
	err   error
}

func streamCopy(dst io.WriteCloser, src io.Reader, c chan *copyRet) {
	n, err := io.Copy(dst, src)
	dst.Close()
	c <- &copyRet{n, err}
}

// Tunnel Handler
func AgentStreamHandler(stream *spdystream.Stream) {
	conn, err := net.Dial("unix", *serviceAddr)
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

	c := make(chan *copyRet, 2)

	go streamCopy(stream, conn, c)
	go streamCopy(conn, stream, c)

	// wait until the copy routine ended
	for i := 0; i < 2; i++ {
		if r := <-c; r.err != nil {
			log.Printf("Error: %s", r.err)
		}
	}
}
