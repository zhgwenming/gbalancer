// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package daemon

import (
	"fmt"
	"github.com/zhgwenming/gbalancer/utils"
	//"io/ioutil"
	stdlog "log"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
)

const (
	DAEMON_ENV = "__GO_DAEMON_MODE"
)

var (
	DefaultDaemon = NewDaemon()
	log           = NewLogger()
)

type Daemon struct {
	PidFile    string
	Foreground bool
	Restart    bool
	Signalc    chan os.Signal
}

func NewDaemon() *Daemon {
	d := &Daemon{}
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
	if d.PidFile != "" {
		if err := utils.WritePid(d.PidFile); err != nil {
			log.Printf("error: %s\n", err)
			os.Exit(1)
		}
	}
}

func (d *Daemon) cleanPidfile() {
	if d.PidFile != "" {
		if err := os.Remove(d.PidFile); err != nil {
			log.Printf("error to remove pidfile %s:", err)
		}
	}
}

// Start will setup the daemon environment and create pidfile if pidfile is not empty
func (d *Daemon) Start() error {
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

	// background process, all the magic goes here
	if _, child := syscall.Getenv(DAEMON_ENV); child {
		syscall.Unsetenv(DAEMON_ENV)
		os.Chdir("/")
		syscall.Setsid()

		d.setupPidfile()

	} else {
		err := syscall.Setenv(DAEMON_ENV, "")
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}

		cmd := exec.Command(os.Args[0], os.Args[1:]...)

		if err = cmd.Start(); err == nil {
			fmt.Printf("- Started daemon as pid %d\n", cmd.Process.Pid)
			os.Exit(0)
		} else {
			fmt.Printf("error to run in daemon mode - %s\n", err)
			os.Exit(1)
		}
	}

	return nil
}

func (d *Daemon) WaitSignal(cleanup func()) {
	// waiting for exit signals
	for sig := range d.Signalc {
		log.Printf("captured %v, exiting..\n", sig)
		// exit if we get any signal
		// Todo - catch signal other than SIGTERM/SIGINT
		break
	}

	// only run hook if it's specified
	if cleanup != nil {
		cleanup()
	}

	d.cleanPidfile()
	return
}

func Start(pidfile string, foreground bool) error {
	DefaultDaemon.PidFile = pidfile
	DefaultDaemon.Foreground = foreground
	return DefaultDaemon.Start()
}

func WaitSignal(cleanup func()) {
	DefaultDaemon.WaitSignal(cleanup)
}
