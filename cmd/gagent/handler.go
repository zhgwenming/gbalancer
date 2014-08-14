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

	go func() {
		go io.Copy(stream, conn)
		go io.Copy(conn, stream)
	}()
	go func() {
		for {
			header, receiveErr := stream.ReceiveHeader()
			if receiveErr != nil {
				return
			}
			sendErr := stream.SendHeader(header, false)
			if sendErr != nil {
				return
			}
		}
	}()
}
