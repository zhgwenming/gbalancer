// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"bytes"
	libvirt "github.com/alexzorin/libvirt-go"
	"log"
	"net"
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
	//networks = make([]*Network, 0, 2)
	networks = make(map[string]Network)
)

func main() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 {
			ifi := iface
			n := "vnet-" + ifi.Name
			net := Network{n, &ifi}
			networks[n] = net
			log.Printf("%s", ifi.Name)
		}
	}

	virConn, err := libvirt.NewVirConnection("lxc:///")

	if err != nil {
		log.Fatal(err)
	}

	// VIR_CONNECT_LIST_NETWORKS_TRANSIENT
	// INACTIVE/ACTIVE
	virNets, err := virConn.ListAllNetworks(libvirt.VIR_CONNECT_LIST_NETWORKS_INACTIVE)
	for _, v := range virNets {
		desc, _ := v.GetXMLDesc(0)
		log.Printf("%v", desc)
	}

	buf := make([]byte, 0, 64)
	xml := bytes.NewBuffer(buf)
	tmpl := template.Must(template.New("net").Parse(VirtNetTemplate))
	for _, net := range networks {
		tmpl.Execute(xml, net)
	}

	log.Printf("%s", xml)
}
