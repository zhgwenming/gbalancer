// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package log

import (
	"log"
	"log/syslog"
	"os"
)

var GlobalLog *log.Logger

func NewLogger(logdir string) (l *log.Logger) {
        if logdir != ""{
 		if filename, err := os.OpenFile(logdir, os.O_CREATE|os.O_APPEND|os.O_RDWR,0660); err != nil {
                        log.Fatal("Create specify log file failure:", err)
                } else {
                        l = log.New(filename, "", log.LstdFlags)
                }
        } else {
		// try to use syslog first
		if logger, err := syslog.NewLogger(syslog.LOG_NOTICE, 0); err != nil {
			l = log.New(os.Stderr, "", log.LstdFlags)
		} else {
			l = logger
		}
	}
	return
}
