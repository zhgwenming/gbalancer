// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"github.com/zhgwenming/gbalancer/utils/libvirt"
)

func main() {
	// a bug exist in virt-sandbox, if we specify a non-exist command
	// such as '111', virt-sandbox will block forever
	libvirt.RunSandbox("/bin/bash")
}
