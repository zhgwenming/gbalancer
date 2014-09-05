// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package native

import (
	"container/heap"
	//splice "github.com/creack/go-splice"
	"github.com/zhgwenming/gbalancer/utils"
	"io"
	"net"
	"sort"
)

type Request struct {
	Conn    net.Conn
	backend *Backend
	err     error
}

type Forwarder struct {
	backend *Backend
	request *Request
	bytes   uint
}

type Scheduler struct {
	pool          Pool
	backendSeq    uint
	backends      map[string]*Backend
	done          chan *Request // to use heap to schedule
	pending       []*Request
	tunnels       uint
	newTunnelChan chan *spdySession
	spdyFailChan  chan *spdySession
}

// it's a leastweight heap if we do persistent scheduling
func NewScheduler(lw bool, tunnels uint) *Scheduler {
	pool := Pool{make([]*Backend, 0, MaxForwarders), lw}
	backends := make(map[string]*Backend, MaxBackends)

	done := make(chan *Request, MaxForwarders)
	pending := make([]*Request, 0, MaxForwarders)

	readyChan := make(chan *spdySession, MaxBackends)
	failChan := make(chan *spdySession, MaxBackends)

	scheduler := &Scheduler{pool, 0, backends, done, pending, tunnels, readyChan, failChan}
	return scheduler
}

func (s *Scheduler) nextBackendSequence() uint {
	s.backendSeq += 1
	return s.backendSeq
}

func (s *Scheduler) Schedule(job chan *Request, status <-chan map[string]int) {
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

			// the NEW active backends
			var addrs []string
			for addr := range backends {
				addrs = append(addrs, addr)
			}

			// 1. shuffle them first if needed
			if *shuffle {
				addrs = utils.Shuffle(addrs)
			} else {
				sort.Strings(addrs)
			}

			// 2. add them to scheduler
			for _, addr := range addrs {
				var weight uint
				if *shuffle {
					weight = 0
				} else {
					weight = s.nextBackendSequence()
				}
				b := NewBackend(addr, s.tunnels, weight)
				//b.failChan = &s.spdyFailChan
				b.FailChan(s.spdyFailChan)
				if s.tunnels > 0 {
					for i := uint(0); i < s.tunnels; i++ {
						go CreateSpdySession(NewSpdySession(b, i), s.newTunnelChan)
					}
				} else {
					s.AddBackend(b)
				}
			}

		case session := <-s.spdyFailChan:
			backend := session.backend
			index := session.connindex
			if !backend.tunnel[index].switching {
				backend.tunnel[index].switching = true
				go CreateSpdySession(session, s.newTunnelChan)
			}
		case session := <-s.newTunnelChan:
			b := session.backend
			// switch the spdy connection first
			b.SwitchSpdyConn(session.connindex, session.spdy)

			if _, ok := s.backends[b.address]; !ok {
				// a new backend, add it to the hash
				s.AddBackend(b)
				// drain the pending list
				if len(s.pending) > 0 && len(s.pool.backends) > 0 {
					for _, p := range s.pending {
						s.dispatch(p)
					}
					s.pending = s.pending[0:0]
				}
			}
		case j := <-job:
			// add to pending list
			if len(s.pool.backends) == 0 {
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
		req.Conn.Close()
		log.Printf("all backend forwarders exceed %d\n", MaxForwardersPerBackend)
		return
	}

	b.ongoing++

	heap.Push(&s.pool, b)
	b.SpdyCheckStreamId(s.newTunnelChan)
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

func sockCopy(dst io.WriteCloser, src io.Reader, c chan *copyRet) {
	n, err := io.Copy(dst, src)
	//log.Printf("sent %d bytes to server", n)

	// make backend read stream ended

	//conn := dst.(net.Conn)
	//conn.SetReadDeadline(time.Now())

	// Close the upstream connection as Deadline
	// not yet supported by spdystream by now
	dst.Close()
	c <- &copyRet{n, err}
}

func (s *Scheduler) run(req *Request) {
	// do the actuall work
	srv, err := req.backend.ForwarderNewConnection(req)
	if err != nil {
		req.err = err
		s.done <- req
		return
	}

	// no need to defer close the upstream server as sockCopy will do that
	// defer srv.Close()

	c := make(chan *copyRet, 2)
	//log.Printf("splicing socks")
	go sockCopy(req.Conn, srv, c)
	go sockCopy(srv, req.Conn, c)

	for i := 0; i < 2; i++ {
		if r := <-c; r.err != nil {
			req.err = err
		}
	}

	s.done <- req
}

func (s *Scheduler) finish(req *Request) {
	backend, err := req.backend, req.err

	if err != nil {
		// keep it out of the heap
		if backend.index != -1 {
			heap.Remove(&s.pool, backend.index)
		}
		backend.ongoing--

		// retry the connection is it failed on dial
		if e, ok := err.(*net.OpError); ok && e.Op == "dial" {
			// detected the connection error
			// keep it out of the heap and try to reschedule the job
			log.Printf("%s, rescheduling request %v\n", err, req)
			s.dispatch(req)
		}
	} else {
		if backend.index == -1 {
			// in case the wrangler already detected error of this backend
			// which makes this backend already removed from the heap pool
			backend.ongoing--
		} else {
			heap.Remove(&s.pool, backend.index)
			backend.ongoing--
			heap.Push(&s.pool, backend)
		}
	}
}

func (s *Scheduler) AddBackend(b *Backend) {
	addr := b.address
	log.Printf("balancer: bring up %s.\n", addr)
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
