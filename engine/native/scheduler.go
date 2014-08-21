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
	"net/http"
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
	pool            Pool
	backends        map[string]*Backend
	done            chan *Request // to use heap to schedule
	pending         []*Request
	useTunnel       bool
	spdyMonitorChan chan *spdySession
	newSessionChan  chan *spdySession
}

// it's a max heap if we do persistent scheduling
func NewScheduler(max, tunnel bool) *Scheduler {
	pool := Pool{make([]*Backend, 0, MaxForwarders), max}
	backends := make(map[string]*Backend, MaxBackends)

	done := make(chan *Request, MaxForwarders)
	pending := make([]*Request, 0, MaxForwarders)

	requestSession := make(chan *spdySession, MaxBackends)
	readySession := make(chan *spdySession, MaxBackends)

	if tunnel {
		go SpdySessionManager(requestSession, readySession)
	}

	scheduler := &Scheduler{pool, backends, done, pending, tunnel, requestSession, readySession}
	return scheduler
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

			var addrs []string
			// the rest of active backends, add them
			for addr := range backends {
				addrs = append(addrs, addr)
			}

			addrs = utils.Shuffle(addrs)
			for _, addr := range addrs {
				b := NewBackend(addr)
				if s.useTunnel {
					s.spdyMonitorChan <- NewSpdySession(b)
				} else {
					s.AddBackend(b)
				}
			}

		case session := <-s.newSessionChan:
			b := session.backend
			if _, ok := s.backends[b.address]; !ok {
				s.AddBackend(b)
				// drain the pending list
				if len(s.pending) > 0 && len(s.pool.backends) > 0 {
					for _, p := range s.pending {
						s.dispatch(p)
					}
					s.pending = s.pending[0:0]
				}
			} else {
				b.SwitchSpdyConn(session.spdy)
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

func (s *Scheduler) NewConnection(req *Request) (net.Conn, error) {
	var conn net.Conn
	var err error

	spdyConn := req.backend.spdyconn

	if spdyConn != nil {
		conn, err = spdyConn.CreateStream(http.Header{}, nil, false)
		if err != nil {
			conn, err = net.Dial("tcp", req.backend.address)
		}
	} else {
		conn, err = net.Dial("tcp", req.backend.address)
	}

	return conn, err
}

func (s *Scheduler) run(req *Request) {
	// do the actuall work
	srv, err := s.NewConnection(req)
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
