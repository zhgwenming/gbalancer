// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package utils

import (
	"fmt"
	logger "github.com/zhgwenming/gbalancer/log"
	"os/exec"
	"strings"
)

var (
	log = logger.NewLogger()
)

func RunCommand(cmd string) error {
	args := strings.Split(cmd, " ")
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Err: %s Output: %s, Cmd %s", err, output, cmd)
		log.Printf("%s", err)
	}
	return err
}

func EnsureCommands(cmds []string) error {
	for _, c := range cmds {
		if err := RunCommand(c); err != nil {
			return err
		}
	}
	return nil
}
