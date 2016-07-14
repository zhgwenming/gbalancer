// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ENV_SUPERVISOR = "__GO_SUPERVISOR_MODE"
)

var (
	DefaultSupervisor = NewSupervisor()
)

type Supervisor struct {
	Daemon
}

func NewSupervisor() *Supervisor {
	d := NewDaemon()
	return &Supervisor{*d}
}

func (s *Supervisor) startWorker() {
	cmd := s.Command

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err == nil {
		log.Printf("- Started worker as pid %d\n", cmd.Process.Pid)
	} else {
		log.Printf("error to start worker - %s\n", err)
		os.Exit(1)
	}

	for sig := range s.Signalc {
		log.Printf("monitor captured %v\n", sig)
		if sig == syscall.SIGCHLD {
			break
		}

		// only exit if we got a TERM signal
		if sig == syscall.SIGTERM {
			cmd.Process.Signal(sig)
			os.Exit(0)
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("worker[%d] exited with - %s\n", cmd.Process.Pid, err)
	}

}

func (s *Supervisor) supervise() {
	signal.Notify(s.Signalc,
		syscall.SIGCHLD)

	// process manager
	for {
		startTime := time.Now()
		s.startWorker()
		for {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			seconds := duration.Seconds()

			// restart for every 5s
			if seconds > 5 {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
		log.Printf("restarting worker\n")
	}
}

func (s *Supervisor) Sink() error {
	mode := os.Getenv(ENV_SUPERVISOR)

	switch mode {
	case "":
		if err := s.Daemon.Sink(); err != nil {
			return err
		}

		// as a foreground process, but give daemon a chance to
		// setup signal/pid related things
		if s.Foreground {
			return nil
		}

		// we should be session leader here
		if err := os.Setenv(ENV_SUPERVISOR, "worker"); err != nil {
			fatal(err)
		}
		s.supervise()
		log.Fatal("BUG, supervisor should loop forever") //should never get here
	case "worker":
		if err := unsetenv(ENV_SUPERVISOR); err != nil {
			fatal(err)
		}

	default:
		err := fmt.Errorf("critical error, unknown mode: %s", mode)
		fmt.Println(err)
		log.Println(err)
		os.Exit(1)
	}

	return nil
}

func Handle(pidfile string, foreground bool, h Handler) SinkServer {
	DefaultSupervisor.h = h
	DefaultSupervisor.PidFile = pidfile
	DefaultSupervisor.Foreground = foreground
	return DefaultSupervisor
}

func HandleFunc(pidfile string, foreground bool, f func()) SinkServer {
	h := HandlerFunc(f)
	return Handle(pidfile, foreground, h)
}
