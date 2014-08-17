// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package daemon

import (
	"flag"
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"github.com/zhgwenming/gbalancer/utils"
	"os"
)

var (
	pidFile = flag.String("pidfile", "", "pid file")
	log     = logger.NewLogger()
)

func CreatePidfile() {
	if *pidFile != "" {
		if err := utils.WritePid(*pidFile); err != nil {
			fmt.Printf("error: %s\n", err)
			log.Fatal("error:", err)
		}
	}
}

func RemovePidfile() {
	if *pidFile != "" {
		if err := os.Remove(*pidFile); err != nil {
			log.Printf("error to remove pidfile %s:", err)
		}
	}
}
