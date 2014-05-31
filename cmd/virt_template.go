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

type VirNetInfo struct {
	Name  string
	Iface *net.Interface
}
type VirNet struct {
	VirNetInfo
	Xml *bytes.Buffer
}

var (
	//networks = make([]*Network, 0, 2)
	RequiredNetwork = make(map[string]VirNet)
)

func main() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	// Compile template first
	tmpl := template.Must(template.New("net").Parse(VirtNetTemplate))

	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 {
			ifi := iface
			log.Printf("%s", ifi.Name)

			// network name
			name := "vnet-" + ifi.Name

			// xml buffer
			buf := make([]byte, 0, 64)
			xml := bytes.NewBuffer(buf)

			// netinfo
			netinfo := VirNetInfo{name, &ifi}
			tmpl.Execute(xml, netinfo)
			//log.Printf("%s", xml)

			virnet := VirNet{netinfo, xml}
			RequiredNetwork[name] = virnet
		}
	}

	virConn, err := libvirt.NewVirConnection("lxc:///")
	if err != nil {
		log.Fatal(err)
	}

	// VIR_CONNECT_LIST_NETWORKS_TRANSIENT
	// INACTIVE/ACTIVE
	inactiveNet, err := virConn.ListAllNetworks(libvirt.VIR_CONNECT_LIST_NETWORKS_INACTIVE)
	for _, v := range inactiveNet {
		name, err := v.GetName()
		if err != nil {
			log.Printf("Error to get libvirt network name: %s", err)
		}
		//desc, _ := v.GetXMLDesc(0)
		//log.Printf("%v", desc)

		if _, ok := RequiredNetwork[name]; ok {
			log.Printf("Undefine inactive network %s", name)
			// undefine an inactive required network
			v.Undefine()
		}
	}

	activeNet, err := virConn.ListAllNetworks(libvirt.VIR_CONNECT_LIST_NETWORKS_ACTIVE)
	for _, v := range activeNet {
		name, err := v.GetName()
		if err != nil {
			log.Printf("Error to get libvirt network name: %s", err)
		}

		if _, ok := RequiredNetwork[name]; ok {
			log.Printf("Found active network %s", name)
			// required network is active, delete it from our active hash
			delete(RequiredNetwork, name)
		}
	}

	// create all required network
	for _, v := range RequiredNetwork {
		xml := v.Xml.String()
		if definedNet, err := virConn.NetworkDefineXML(xml); err != nil {
			log.Printf("Define virt network error: %s", err)
		} else {
			if err = definedNet.Create(); err != nil {
				log.Printf("Create virt network error: %s", err)
			} else {
				log.Printf("Start network %s", v.Name)
			}
		}
	}

}
