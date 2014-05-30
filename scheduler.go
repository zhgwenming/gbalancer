// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"container/heap"
	//splice "github.com/creack/go-splice"
	"io"
	//"log"
	"net"
	"time"
)

type Scheduler struct {
	pool     Pool
	backends map[string]*Backend
	done     chan *Request // to use heap to schedule
	pending  []*Request
}

func NewScheduler() *Scheduler {
	pool := make(Pool, 0, MaxForwarders)
	backends := make(map[string]*Backend, MaxBackends)
	done := make(chan *Request, MaxForwarders)
	pending := make([]*Request, 0, MaxForwarders)
	scheduler := &Scheduler{pool, backends, done, pending}
	return scheduler
}

func (s *Scheduler) schedule(job chan *Request, status <-chan map[string]int) {
	for {
		select {
		case back := <-s.done:
			//log.Println("finishing a connection")
			s.finish(back)
		case backends := <-status:
			if len(backends) == 0 {
				log.Printf("balancer: got empty backends list")
			}

			for addr, b := range s.backends {
				if _, ok := backends[addr]; !ok {
					// not exist in the active backend list
					s.RemoveBackend(addr)
				} else {
					delete(backends, addr)
					// push back backend with error in run()
					if b.index == -1 {
						log.Printf("balancer: bring back %s to up\n", b.address)
						heap.Push(&s.pool, s.backends[addr])
					}
				}
			}

			// the rest of active backends, add them
			for addr, _ := range backends {
				s.AddBackend(addr)
			}

			// drain the pending list
			if len(s.pending) > 0 && len(s.pool) > 0 {
				for _, p := range s.pending {
					s.dispatch(p)
				}
				s.pending = s.pending[0:0]
			}
		case j := <-job:
			// add to pending list
			if len(s.pool) == 0 {
				s.pending = append(s.pending, j)
				log.Printf("No backend available\n")
				continue
			}
			//log.Println("Got a connection")

			s.dispatch(j)
		}

	}
}

func (s *Scheduler) dispatch(req *Request) {
	b := heap.Pop(&s.pool).(*Backend)
	if b.ongoing >= MaxForwardersPerBackend {
		heap.Push(&s.pool, b)
		req.conn.Close()
		log.Printf("all backend forwarders exceed %d\n", MaxForwardersPerBackend)
		return
	}

	b.ongoing++
	heap.Push(&s.pool, b)
	req.backend = b
	go s.run(req)
}

type copyRet struct {
	bytes int64
	err   error
}

//func spliceCopy(dst io.Writer, src io.Reader, c chan *copyRet) {
//	n, err := splice.Copy(dst, src)
//	c <- &copyRet{n, err}
//}

func sockCopy(dst io.Writer, src io.Reader, c chan *copyRet) {
	n, err := io.Copy(dst, src)
	//log.Printf("sent %d bytes to server", n)
	// make backend read stream ended
	conn := dst.(net.Conn)
	conn.SetReadDeadline(time.Now())
	c <- &copyRet{n, err}
}

func (s *Scheduler) run(req *Request) {
	// do the actuall work
	srv, err := net.Dial("tcp", req.backend.address)
	if err != nil {
		req.err = err
		return
	}
	defer srv.Close()

	c := make(chan *copyRet, 2)
	//log.Printf("splicing socks")
	go sockCopy(req.conn, srv, c)
	go sockCopy(srv, req.conn, c)

	for i := 0; i < 2; i++ {
		if r := <-c; r.err != nil {
			req.err = err
		}
	}

	s.done <- req
}

func (s *Scheduler) finish(req *Request) {
	backend, err := req.backend, req.err

	// retry the connection is it failed on dial
	if e, ok := err.(*net.OpError); ok && e.Op == "dial" {
		// detected the connection error
		// keep it out of the heap and try to reschedule the job
		if backend.index != -1 {
			heap.Remove(&s.pool, backend.index)
		}
		backend.ongoing--
		log.Printf("%s, rescheduling reqest %v\n", err, req)
		s.dispatch(req)
	} else {
		heap.Remove(&s.pool, backend.index)
		backend.ongoing--
		heap.Push(&s.pool, backend)
		req.conn.Close()
	}
}

func (s *Scheduler) AddBackend(addr string) {
	log.Printf("balancer: bring up %s.\n", addr)
	b := NewBackend()
	b.address = addr
	s.backends[addr] = b
	heap.Push(&s.pool, b)
}

func (s *Scheduler) RemoveBackend(addr string) {
	log.Printf("balancer: take down %s.\n", addr)
	if b, ok := s.backends[addr]; ok {
		// the backend might be already removed from the heap
		if b.index != -1 {
			heap.Remove(&s.pool, b.index)
		}
		delete(s.backends, b.address)
	} else {
		log.Printf("balancer: %s is not up, bug might exist!", addr)
	}

}
