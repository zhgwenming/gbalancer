// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

type Pool struct {
	backends []*Backend
	// max/min heap
	max bool
}

func (p Pool) Len() int {
	return len(p.backends)
}

func (p Pool) Less(i, j int) bool {
	if p.max {
		return p.backends[i].ongoing > p.backends[j].ongoing
	} else {
		return p.backends[i].ongoing < p.backends[j].ongoing
	}
}

func (p *Pool) Swap(i, j int) {
	n := p.backends
	n[i], n[j] = n[j], n[i]
	n[i].index = i
	n[j].index = j
}

func (p *Pool) Push(x interface{}) {
	n := p.backends
	l := len(n)
	n = n[0 : l+1]
	b := x.(*Backend)
	b.index = l
	n[l] = b
	p.backends = n
}

func (p *Pool) Pop() interface{} {
	n := p.backends
	p.backends = n[0 : len(n)-1]
	b := n[len(n)-1]
	b.index = -1
	return b
}
