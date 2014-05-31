// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package log

import (
	"log"
	"log/syslog"
	"os"
)

func NewLogger() (l *log.Logger) {
	// try to use syslog first
	if logger, err := syslog.NewLogger(syslog.LOG_NOTICE, 0); err != nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	} else {
		l = logger
	}
	return
}
