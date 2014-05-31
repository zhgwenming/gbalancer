// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"log"
	"net"
	"os"
	"text/template"
)

const VirtNetTemplate = `
<network>
  <name>{{.Name}}</name>
  <forward mode="bridge">
    <interface dev="{{.Iface.Name}}"/>
  </forward>
</network>
`

type Network struct {
	Name  string
	Iface *net.Interface
}

var (
	networks = make([]*Network, 0, 2)
)

func main() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 {
			ifi := iface
			network := &Network{"vnet-" + ifi.Name, &ifi}
			networks = append(networks, network)
			log.Printf("%s", iface.Name)
		}
	}

	xml := template.Must(template.New("net").Parse(VirtNetTemplate))
	for _, net := range networks {
		xml.Execute(os.Stdout, net)
	}
}
