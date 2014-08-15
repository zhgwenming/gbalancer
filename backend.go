// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/docker/spdystream"
)

const (
	streamPort = 6900
)

type Backend struct {
	spdyconn *spdystream.Connection
	address  string
	flags    int
	index    int
	ongoing  uint
	RxBytes  uint64
	TxBytes  uint64
}

func NewBackend(addr string) *Backend {
	return &Backend{address: addr}
}
