// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"strings"
)

// mysql> show status like 'wsrep_%';
// | wsrep_local_state          | 4                                                     |
// | wsrep_local_state_comment  | Synced                                                |
// | wsrep_cert_index_size      | 753                                                   |
// | wsrep_causal_reads         | 0                                                     |
// | wsrep_incoming_addresses   | 10.100.91.74:3306,10.100.91.72:3306,10.100.91.71:3306 |
// | wsrep_cluster_conf_id      | 27                                                    |
// | wsrep_cluster_size         | 3                                                     |
// | wsrep_cluster_state_uuid   | 068a7c10-780c-11e3-0800-11bf80b0e109                  |
// | wsrep_cluster_status       | Primary                                               |
// | wsrep_connected            | ON                                                    |
// | wsrep_local_index          | 1                                                     |
// | wsrep_provider_name        | Galera                                                |
// | wsrep_provider_vendor      | Codership Oy <info@codership.com>                     |
// | wsrep_provider_version     | 2.5(r147)                                             |
// | wsrep_ready                | ON                                                    |
// +----------------------------+-------------------------------------------------------+

type Galera struct {
	User     string
	Pass     string
	Director []string // directory server, order sensitive, will use the first one by default
}

func NewGalera(user, pass string) *Galera {
	dir := make([]string, 0, MaxBackends)
	return &Galera{user, pass, dir}
}

func (c *Galera) AddDirector(backend string) error {
	c.Director = append(c.Director, backend)
	return fmt.Errorf("Error to add backend %s\n", backend)
}

func galeraProbe(user, pass, host string) (map[string]string, error) {
	// debug purpose
	all := false

	var wsrep_status = map[string]string{
		WsrepConnected: "",
		WsrepAddresses: "",
	}

	// user:password@tcp(db.example.com:3306)/dbname
	dsn := user + ":" + pass + "@tcp(" + host + ")/?timeout=1s"
	//slog.Printf("Probing %s\n", dsn)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		//slog.Printf("%s\n", err)
		return wsrep_status, err
	}
	defer db.Close()

	rows, err := db.Query("show status like 'wsrep_%'")
	if err != nil {
		//slog.Printf("%s\n", err)
		return wsrep_status, err
	}

	for rows.Next() {
		var key string
		var value string
		err = rows.Scan(&key, &value)
		if _, ok := wsrep_status[key]; ok {
			wsrep_status[key] = value
			//if !all {
			//	slog.Printf("%s %s\n", key, value)
			//}
		}
		if all {
			slog.Printf("%s %s\n", key, value)
		}
	}

	err = fmt.Errorf("Galera Not Connected")
	if val, ok := wsrep_status[WsrepConnected]; ok && val == "ON" {
		err = nil
	}
	return wsrep_status, err
}

type backendStatus struct {
	backend string
	err     error
}

// check the backend status
func (c *Galera) BuildActiveBackends() (map[string]int, error) {
	backends := make(map[string]int, MaxBackends)

	if len(c.Director) == 0 {
		return backends, fmt.Errorf("Empty directory server list\n")
	}

	results := make(chan backendStatus, MaxBackends)

	probe := func(user, pass, addr string) {
		_, err := galeraProbe(c.User, c.Pass, addr)
		results <- backendStatus{addr, err}
		//if err != nil {
		//	slog.Printf("probe: %s\n", err)
		//}
	}

	for dirIndex, dirAddr := range c.Director {
		status, err := galeraProbe(c.User, c.Pass, dirAddr)
		if err != nil {
			slog.Println(err)
			continue
		}

		backends[dirAddr] = FlagUp
		if dirIndex != 0 {
			c.Director[0], c.Director[dirIndex] = c.Director[dirIndex], c.Director[0]
			slog.Printf("Make %s as the first director\n", dirAddr)
		}

		if val, ok := status[WsrepAddresses]; ok && val != "" {
			addrs := strings.Split(val, ",")
			numWorkers := 0
			for _, addr := range addrs {
				// director server itself already probed, skip
				if addr == dirAddr {
					continue
				}

				go probe(c.User, c.Pass, addr)
				numWorkers++
			}
			for i := 0; i < numWorkers; i++ {
				r := <-results
				if r.err == nil {
					backends[r.backend] = FlagUp
					//slog.Printf("host: %s\n", r.backend)
				} else {
					slog.Printf("error: %s", r.err)
				}
			}
			break
		} else {
			slog.Printf("host %s: %s key doesn't exist in status\n", dirAddr, WsrepAddresses)
			continue
		}
	}
	//slog.Printf("Active server: %v\n", backends)
	return backends, nil
}
