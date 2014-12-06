package main

import (
	"os"

	"github.com/op/go-logging"
)

type Radio struct {
	Name string
	Uri  string
}

type Server struct {
	Host string
	Port int
}

type Config struct {
	Servers   map[string]*Radio
	Receivers map[string]*Server
}

type Backends struct {
	Radios        []*Radio
	Servers       []*Server
	StaticServers []*Server
	Receivers     []*string
	Names         map[string]*string
}

var (
	log = logging.MustGetLogger("main")
)

func initLogger(verbose bool) {
	format := logging.MustStringFormatter("%{color}%{time:15:04:05} %{level:.6s} ▶ %{shortfunc} %{color:reset} %{message}")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	formatter := logging.NewBackendFormatter(backend, format)
	leveled := logging.AddModuleLevel(formatter)
	if verbose {
		leveled.SetLevel(logging.DEBUG, "")
	} else {
		leveled.SetLevel(logging.INFO, "")
	}
	log.SetBackend(leveled)
}
