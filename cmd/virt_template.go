// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"text/template"
)

const VirtNetTemplate = `
<network>
  <name>{{.Network.Name}}</name>
  <forward mode="bridge">
    <interface dev="{{.Network.Iface.Name}}"/>
  </forward>
</network>
`

func init() {
}
