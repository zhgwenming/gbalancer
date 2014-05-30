// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

type Pool []*Backend

func (p Pool) Len() int {
	return len(p)
}

func (p Pool) Less(i, j int) bool {
	return p[i].ongoing < p[j].ongoing
}

func (p *Pool) Swap(i, j int) {
	n := *p
	n[i], n[j] = n[j], n[i]
	n[i].index = i
	n[j].index = j
}

func (p *Pool) Push(x interface{}) {
	n := *p
	l := len(n)
	n = n[0 : l+1]
	b := x.(*Backend)
	b.index = l
	n[l] = b
	*p = n
}

func (p *Pool) Pop() interface{} {
	n := *p
	*p = n[0 : len(n)-1]
	b := n[len(n)-1]
	b.index = -1
	return b
}
