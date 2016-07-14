// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"fmt"
	"github.com/zhgwenming/gbalancer/utils"
	stdlog "log"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"
)

const (
	ENV_DAEMON = "__GO_DAEMON_MODE"
)

var (
	DefaultDaemon = NewDaemon()
	log           = NewLogger()
)

type Handler interface {
	Serve()
	Stop()
}

type Daemon struct {
	PidFile     string
	Foreground  bool
	Signalc     chan os.Signal
	Command     exec.Cmd
	WaitSeconds time.Duration
	Log         logFile
	h           Handler
}

func NewDaemon() *Daemon {
	d := &Daemon{WaitSeconds: time.Second}
	d.Signalc = make(chan os.Signal, 1)

	return d
}

func NewLogger() (l *stdlog.Logger) {
	// try to use syslog first
	if logger, err := syslog.NewLogger(syslog.LOG_NOTICE, 0); err != nil {
		l = stdlog.New(os.Stderr, "", stdlog.LstdFlags)
	} else {
		l = logger
	}
	return
}

func fatal(err error) {
	log.Printf("error: %s\n", err)
	os.Exit(1)
}

func (d *Daemon) setupPidfile() {
	if d.PidFile == "" {
		return
	}

	if err := utils.WritePid(d.PidFile); err != nil {
		log.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

func (d *Daemon) cleanPidfile() {
	if d.PidFile == "" {
		return
	}

	if err := os.Remove(d.PidFile); err != nil {
		log.Printf("error to remove pidfile %s:", err)
	}
}

func (d *Daemon) createLogfile() (*os.File, error) {
	var err error

	if d.Log.path == "" {
		logfile := "/tmp/" + path.Base(os.Args[0]) + ".log"
		d.Log.path = logfile
	}

	if err = d.Log.Open(); err != nil {
		fmt.Printf("- Failed to create output log file - %s: %s\n", d.Log.path, err)
	}

	if err != nil {
		return nil, err
	} else {
		return d.Log.file, nil
	}
}

// monitor or the worker process
func (d *Daemon) child() {
	os.Chdir("/")

	// Setsid in the exec.Cmd.SysProcAttr.Setsid
	//syscall.Setsid()

	d.setupPidfile()
}

func (d *Daemon) parent() {
	signal.Notify(d.Signalc,
		syscall.SIGCHLD)

	cmd := d.Command

	procAttr := &syscall.SysProcAttr{Setsid: true}
	cmd.SysProcAttr = procAttr

	if file, err := d.createLogfile(); err == nil {
		fmt.Printf("- redirected the output to %s\n", file.Name())
		cmd.Stdout = file
		cmd.Stderr = file
	}

	if err := cmd.Start(); err == nil {
		fmt.Printf("- Started daemon as pid %d\n", cmd.Process.Pid)
		select {
		case <-time.After(time.Second / 5):
		case sig := <-d.Signalc:
			if sig == syscall.SIGCHLD {
				if err := cmd.Wait(); err != nil {
					fmt.Printf("- daemon exited with %s\n", err)
					d.Log.Dump(os.Stderr)
				}
			}
		}
		os.Exit(0)
	} else {
		fmt.Printf("error to run in daemon mode - %s\n", err)
		os.Exit(1)
	}
}

// RunWait will run the specified function in safe mode, it blocks the caller until it finished
func (d *Daemon) RunWait(handler func()) error {
	if p := recover(); p != nil {
		log.Printf("%s\nbacktrace:\n%s", p, debug.Stack())
	}
	handler()

	return nil
}

func (d *Daemon) runLoop(handler func()) error {
	for {
		startTime := time.Now()
		d.RunWait(handler)
		for {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			if duration.Seconds() > 5 {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
	}
}

// RunForever returns imediately to the caller and run the specified function
// in background, it watches over the requested function in a separate
// goroutine, the function will get restarted infinitely on errors.
func (d *Daemon) RunForever(handler func()) error {
	go d.runLoop(handler)

	return nil
}

// RunForever returns imediately to the caller and run the specified function in background
func (d *Daemon) RunOnce(handler func()) error {
	go d.RunWait(handler)

	return nil
}

// Start will setup the daemon environment and create pidfile if pidfile is not empty
// Parent process will never return
// Will return back to the worker process
func (d *Daemon) Sink() error {
	if d.h == nil {
		return fmt.Errorf("Handler should be specified first")
	}

	// the signal handler is needed for both parent and child
	// since we need to support foreground mode
	signal.Notify(d.Signalc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM)

	if d.PidFile != "" {
		if _, err := os.Stat(path.Dir(d.PidFile)); os.IsNotExist(err) {
			return err
		}

		// switch to use abs pidfile, background daemon will chdir to /
		if p, err := filepath.Abs(d.PidFile); err != nil {
			fatal(err)
		} else {
			d.PidFile = p
		}
	}

	// as a foreground process
	if d.Foreground {
		fmt.Printf("- Running as foreground process\n")
		d.setupPidfile()
		return nil
	}

	if p, err := filepath.Abs(os.Args[0]); err != nil {
		fatal(err)
	} else {
		d.Command = exec.Cmd{
			Path: p,
			Args: os.Args,
		}
	}

	// parent/child/worker logic
	// background monitor/worker process, all the magic goes here
	mode := os.Getenv(ENV_DAEMON)

	switch mode {
	case "":
		err := os.Setenv(ENV_DAEMON, "child")
		if err != nil {
			fatal(err)
		}

		d.parent()                           // fork and exit
		log.Fatal("BUG, parent didn't exit") //should never get here
	case "child":
		if err := unsetenv(ENV_DAEMON); err != nil {
			fatal(err)
		}

		d.child()
	default:
		err := fmt.Errorf("critical error, unknown mode: %s", mode)
		fmt.Println(err)
		log.Println(err)
		os.Exit(1)
	}

	return nil
}

func (d *Daemon) Serve() {
	// handler serve
	d.h.Serve()
}

func (d *Daemon) WaitSignal() {
	// waiting for exit signals
	for sig := range d.Signalc {
		log.Printf("captured %v, exiting..\n", sig)
		// exit if we get any signal
		// Todo - catch signal other than SIGTERM/SIGINT
		break
	}

	// handler stop routine
	d.h.Stop()

	d.cleanPidfile()
	return
}

type HandlerFunc func()

func (h HandlerFunc) Serve() {
	h()
}

func (h HandlerFunc) Stop() {
	return
}

func (d *Daemon) Handle(h Handler) {
	d.h = h
}

func (d *Daemon) HandleFunc(f func()) {
	d.h = HandlerFunc(f)
}

type SinkServer interface {
	Sink() error
	Serve()
	WaitSignal()
}

func DaemonHandle(pidfile string, foreground bool, h Handler) SinkServer {
	DefaultDaemon.h = h
	DefaultDaemon.PidFile = pidfile
	DefaultDaemon.Foreground = foreground
	return DefaultDaemon
}

func DaemonHandleFunc(pidfile string, foreground bool, f func()) SinkServer {
	h := HandlerFunc(f)
	return DaemonHandle(pidfile, foreground, h)
}

// a function calls different sink functions
func Start(s SinkServer) error {

	if err := s.Sink(); err != nil {
		return err
	}

	// handler serve
	s.Serve()

	// wait to exit
	s.WaitSignal()
	return nil
}
