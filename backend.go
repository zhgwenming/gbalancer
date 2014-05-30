// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

type Backend struct {
	address string
	flags   int
	index   int
	ongoing uint
	RxBytes uint64
	TxBytes uint64
}

func NewBackend() *Backend {
	return &Backend{}
}
