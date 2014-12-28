package main

import (
	"flag"
	"os"
)

func main() {
	hostname, _ := os.Hostname()
	m := NewServer(false)
	flag.StringVar(&m.Server().Name, "name", hostname, "server name")
	flag.StringVar(&m.Server().Host, "host", hostname, "server host name")
	flag.IntVar(&m.Server().Port, "port", 48100, "server port")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.StringVar(&m.StaticUri, "uri", "", "uri to stream into the network")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	if m.StaticUri == "" {
		log.Error("--uri is mandatory")
		os.Exit(1)
	}

	m.startSender()
}
