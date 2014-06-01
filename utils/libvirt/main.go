// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package libvirt

import (
	"github.com/zhgwenming/gbalancer/utils"
	"log"
)

func RunSandbox() {
	CreateRequiredNetwork()
	sandbox := utils.NewSandbox("test-sandbox", "vnet-eno16777736", "/bin/bash")
	sandbox.Addr = "172.16.154.199"
	err := sandbox.Run()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Ended")
}
