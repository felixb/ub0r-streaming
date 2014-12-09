package main

import (
	"os"
	"time"

	"github.com/op/go-logging"
)

type Pinger interface {
	Ping()
}

type Radio struct {
	Name string
	Uri  string
}

type Server struct {
	Name     string
	Host     string
	Port     int
	LastPing int64
}

type Receiver struct {
	Name     string
	Host     string
	LastPing int64
}

// .Host is key in this map
type Config struct {
	Servers   map[string]*Radio
	Receivers map[string]*Server
}

type Backends struct {
	Radios        []*Radio
	Servers       []*Server
	StaticServers []*Server
	Receivers     []*Receiver
}

var (
	log            = logging.MustGetLogger("main")
	backendTimeout = 1 * time.Minute
)

// ----- interfaces -------------------------------

func (e *Server) Ping() {
	e.LastPing = time.Now().Unix()
}

func (e *Receiver) Ping() {
	e.LastPing = time.Now().Unix()
}

// ----- logging -------------------------------

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
