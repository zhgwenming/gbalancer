// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package utils

type Sandbox struct {
	Name    string
	Network string
	Command string
	Args    []string
	Addr    string
}

// virt-sandbox /bin/bash --network address=172.16.154.199/24,source=lan  -n test-virt-sandbox
func NewSandbox(name, network, cmd string) *Sandbox {
	args := make([]string, 0)
	return &Sandbox{name, network, cmd, args, ""}
}

func (s *Sandbox) RunService() error {
	//cmdline := "virt-sandbox --network address=" + s.Addr + "/24,source=" + s.Network + "  -n " + s.Name + " " + s.Command
	cmdline := "virt-sandbox-service create --network address=" + s.Addr + "/24,source=" + s.Network + " " + s.Name + " -- " + s.Command
	cmds := []string{
		cmdline,
		"virsh start " + s.Name,
	}
	return EnsureCommands(cmds)
}

func (s *Sandbox) Run() error {
	cmdline := "virt-sandbox --network address=" + s.Addr + "/24,source=" + s.Network + "  -n " + s.Name + " " + s.Command
	return RunCommand(cmdline)
}
