// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package wrangler

const (
	MaxBackends             uint = 128
	MaxForwardersPerBackend uint = 4096
)

const (
	WsrepAddresses = "wsrep_incoming_addresses"
	WsrepConnected = "wsrep_connected"
	CheckInterval  = 60
)

const (
	FlagDown int = 0
	FlagUp   int = 1
)
