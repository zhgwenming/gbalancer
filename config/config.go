// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DEFAULT_UNIX_SOCKET = "/var/lib/mysql/mysql.sock"
)

func CheckFile(cfg string) error {
	if filepath.IsAbs(cfg) {
		_, err := os.Stat(cfg)
		os.IsNotExist(err)
		return err
	} else {
		return fmt.Errorf("config file need to be specified with absolute path - %s", cfg)
	}
}

func LoadConfig(configFile string) (*Configuration, error) {
	file, err := os.Open(configFile)

	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(file)
	config := &Configuration{
		Service:  "galera",
		Addr:     "127.0.0.1",
		Port:     "3306",
		Timeout:  "5",
	}

	err = decoder.Decode(config)

	// for compatible reason, may remove in the future
	// might be needed by the ipvs engine
	if config.Addr != "" {
		tcpAddr := "tcp://" + config.Addr + ":" + config.Port
		config.AddListen(tcpAddr)
	}

	return config, err
}

type Configuration struct {
	Service    string
	ExtCommand string
	User       string
	Pass       string
	Addr       string
	Port       string
	Timeout    string
	Listen     []string
	Backend    []string
}

func (c *Configuration) ListenInfo() string {
	return fmt.Sprintf("Listen on %v, backend: %v", c.Listen, c.Backend)
}

func (c *Configuration) AddListen(listen string) {
	c.Listen = append(c.Listen, listen)
}

func (c *Configuration) GetListenAddrs() ([]ListenAddr, error) {
	laddrs := make([]ListenAddr, 0, len(c.Listen))
	for _, l := range c.Listen {
		protoAddrParts := strings.SplitN(l, "://", 2)
		if len(protoAddrParts) != 2 {
			err := fmt.Errorf("incorrect listen addr %s", l)
			return laddrs, err
		}

		net, laddr := protoAddrParts[0], protoAddrParts[1]

		var addr ListenAddr
		if net == "unix" {
			// unix://default form
			if laddr == "/" || laddr == "default" {
				laddr = DEFAULT_UNIX_SOCKET
			}
		}

		addr = ListenAddr{net, laddr}

		laddrs = append(laddrs, addr)
	}

	return laddrs, nil
}
