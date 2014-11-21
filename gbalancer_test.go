package main

import (
	"io"
	"io/ioutil"
	"net/http"
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

func start() {
	ioutil.WriteFile(cfgFile, configTemplate, 0600)

	args := []string{"-config", cfgFile}
	os.Args = append(os.Args, args...)
	main()
}

func startHTTP(port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello\n")
	})

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServer:", err)
	}
}

func TestMain(t *testing.T) {
	go startHTTP("9001")
	start()
}
