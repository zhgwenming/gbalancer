package main

import (
	"io/ioutil"
	"os"
	"testing"
)

const cfgFile = "/tmp/gbalancer.json"

var configTemplate = []byte(`
{
    "service": "http",
    "addr": "127.0.0.1",
    "port": "9000",
    "listen": [
	"unix:///tmp/mysql.sock"
    ],
    "backend": [
        "127.0.0.1:9001",
        "127.0.0.1:9002",
        "127.0.0.1:9003"
    ]
}
`)

func TestMain(t *testing.T) {
	ioutil.WriteFile(cfgFile, configTemplate, 0600)

	args := []string{"-config", cfgFile}
	os.Args = append(os.Args, args...)
	main()
}
