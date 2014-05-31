// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package wrangler

import (
	//"log"
	"testing"
)

//mysql -u monitor -p3FNg6IKe

func TestCheck(t *testing.T) {
	//log.Printf("hello\n")
	back := make(chan *BEStatus, MaxBackends)
	g := NewGaleraChecker("monitor", "3FNg6IKe", back)
	server1 := "11.100.91.72:3306"
	server2 := "10.100.91.71:3306"
	g.AddDirector(&server1)
	g.AddDirector(&server2)
	//g.BuildActiveBackends()
	g.Monitor()
}
