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
	logger "github.com/zhgwenming/gbalancer/log"
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
	logger.GlobalLog.Printf("Test_Issue: start scheduler is finished successfully\n")
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
			logger.GlobalLog.Println("finishing a connection")
			s.finish(back)
		case backends := <-status:
			if len(backends) == 0 {
				logger.GlobalLog.Printf("balancer: got empty backends list")
			}

			for addr, b := range s.backends {
				if _, ok := backends[addr]; !ok {
					// not exist in the active backend list
					logger.GlobalLog.Printf("Test_Issue: Schedule remove no active backend: %s\n", addr)
					s.RemoveBackend(addr)
				} else {
					logger.GlobalLog.Printf("Test_Issue: Schedule delete specified backend: %s\n", addr)
					delete(backends, addr)
					// push back backend with error in run()
					if b.index == -1 {
						logger.GlobalLog.Printf("balancer: bring back %s to up\n", b.address)
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
						logger.GlobalLog.Printf("Test_Issue: Schedule execution NewSpdySession\n")
						go CreateSpdySession(NewSpdySession(b, i), s.newTunnelChan)
					}
				} else {
					s.AddBackend(b)
				}
			}
			logger.GlobalLog.Printf("Test_Issue: Schedule NEW active backend\n")

		case session := <-s.spdyFailChan:
			logger.GlobalLog.Printf("Test_Issue: Schedule spdyFailchan\n")
			backend := session.backend
			index := session.connindex
			if !backend.tunnel[index].switching {
				backend.tunnel[index].switching = true
				go CreateSpdySession(session, s.newTunnelChan)
			}
		case session := <-s.newTunnelChan:
			logger.GlobalLog.Printf("Test_Issue: Schedule add new backend to the hash\n")
			b := session.backend
			// switch the spdy connection first
			b.SwitchSpdyConn(session.connindex, session.spdy)

			if _, ok := s.backends[b.address]; !ok {
				// a new backend, add it to the hash
				s.AddBackend(b)
				// drain the pending list
				if len(s.pending) > 0 && len(s.pool.backends) > 0 {
		            logger.GlobalLog.Printf("Test_Issue: Execution dispatch in newTunnelChan item of Schedule function\n")
					for _, p := range s.pending {
						s.dispatch(p)
					}
					s.pending = s.pending[0:0]
				}
			}
		case j := <-job:
		    logger.GlobalLog.Printf("Test_Issue: Execution dispatch in job item of Schedule function\n")
			s.dispatch(j)
		}

	}
}

// dispatch or add to pending list
func (s *Scheduler) dispatch(req *Request) {
	// add to pending list
	if len(s.pool.backends) == 0 {
		s.pending = append(s.pending, req)
		logger.GlobalLog.Printf("No backend available\n")
		return
	}
	logger.GlobalLog.Println("scheduler dispatch: Got a connection")

	b := heap.Pop(&s.pool).(*Backend)
	if b.ongoing >= MaxForwardersPerBackend {
		heap.Push(&s.pool, b)
		req.Conn.Close()
		logger.GlobalLog.Printf("all backend forwarders exceed %d\n", MaxForwardersPerBackend)
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
	//logger.GlobalLog.Printf("sent %d bytes to server", n)

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
	logger.GlobalLog.Printf("Test_Issue: Schedule run: do the actuall work\n")

	srv, err := req.backend.ForwarderNewConnection(req)
	if err != nil {
		req.err = err
		s.done <- req
		return
	}

	// no need to defer close the upstream server as sockCopy will do that
	// defer srv.Close()

	c := make(chan *copyRet, 2)
	logger.GlobalLog.Printf("splicing socks")
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
		logger.GlobalLog.Printf("Test_Issue: Schedule finish: keep it out of the heap\n")
		if backend.index != -1 {
			heap.Remove(&s.pool, backend.index)
		}
		backend.ongoing--

		// retry the connection is it failed on dial
		if e, ok := err.(*net.OpError); ok && e.Op == "dial" {
			// detected the connection error
			// keep it out of the heap and try to reschedule the job
			logger.GlobalLog.Printf("%s, rescheduling request %v\n", err, req)
		    logger.GlobalLog.Printf("Test_Issue: Execution dispatch in finish function\n")
			s.dispatch(req)
		}
	} else {
		if backend.index == -1 {
			// in case the wrangler already detected error of this backend
			// which makes this backend already removed from the heap pool
			logger.GlobalLog.Printf("Test_Issue: Schedule finish: wrangler already detected error of this backend\n")
			backend.ongoing--
		} else {
			logger.GlobalLog.Printf("Test_Issue: Schedule finish: push back this backend to heap\n")
			heap.Remove(&s.pool, backend.index)
			logger.GlobalLog.Printf("Test_Issue: Schedule finish: Tunnels are %d\n", backend.tunnels)
			//for i := 0; i < int(backend.tunnels); i++ {
				//oneTunnel := backend.tunnel[i]
				//oneTunnel.Close()
			//}
			backend.ongoing--
			heap.Push(&s.pool, backend)
		}
	}
}

func (s *Scheduler) AddBackend(b *Backend) {
	addr := b.address
	logger.GlobalLog.Printf("Schedule AddBackend balancer: bring up %s.\n", addr)
	s.backends[addr] = b
	heap.Push(&s.pool, b)
}

func (s *Scheduler) RemoveBackend(addr string) {
	logger.GlobalLog.Printf("Schedule RemoveBackend balancer: take down %s.\n", addr)
	if b, ok := s.backends[addr]; ok {
		// the backend might be already removed from the heap
		if b.index != -1 {
			heap.Remove(&s.pool, b.index)
		}
		for i := 0; i < int(b.tunnels); i++ {
			oneTunnel := b.tunnel[i]
			oneTunnel.Close()
		}
		delete(s.backends, b.address)
	} else {
		logger.GlobalLog.Printf("balancer: %s is not up, bug might exist!", addr)
	}

}
