// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package config

import (
	"net"
)

type ListenAddr struct {
	net   string
	laddr string
}

func (l *ListenAddr) Listen() (net.Listener, error) {
	return net.Listen(l.net, l.laddr)
}
