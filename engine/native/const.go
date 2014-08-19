// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

const (
	MaxBackends             uint   = 128
	MaxForwarders           uint   = 8192
	MaxForwardersPerBackend uint   = 4096
	MaxStreamId             uint32 = 0x7fffffff - (0x1 << 20)
)

const (
	ListenAddr          = "127.0.0.1"
	ListenPort          = "3306"
	DEFAULT_UNIX_SOCKET = "/var/lib/mysql/mysql.sock"
)

const (
	FlagDown int = 0
	FlagUp   int = 1
)

const (
	ReqRefused int = 1
)
