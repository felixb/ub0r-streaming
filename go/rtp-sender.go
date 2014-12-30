package main

import (
	"flag"
	"os"
)

func main() {
	hostname, _ := os.Hostname()
	m := NewServer(false)
	s := m.Server()
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.StringVar(&s.Name, "name", hostname, "server name")
	flag.StringVar(&s.Host, "host", hostname, "server host name")
	flag.IntVar(&s.Port, "port", 48100, "server port")
	flag.StringVar(&s.RadioUri, "uri", "", "uri to stream into the network")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	s.RadioId = "static"
	if s.RadioUri == "" {
		log.Error("--uri is mandatory")
		os.Exit(1)
	}

	m.startSender()
}
