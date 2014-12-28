package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"time"

	"github.com/op/go-logging"
)

type Pinger interface {
	Id() string
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

type Config struct {
	// map Server.Id() to active Radio.Id()
	Servers   map[string]string
	// map Recevier.Id() to active Server.Id()
	Receivers map[string]string
	Backends  *Backends
}

type Backends struct {
	Radios        map[string]*Radio
	Servers       map[string]*Server
	StaticServers map[string]*Server
	Receivers     map[string]*Receiver
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

func (s *Server) Id() string {
	return fmt.Sprintf("server-%s:%d", s.Host, s.Port)
}

func (r *Receiver) Id() string {
	return fmt.Sprintf("receiver-%s", r.Host)
}

func (r *Radio) Id() string {
	return fmt.Sprintf("radio-%x", sha1.Sum([]byte(r.Uri)))
}

func (b *Backends) hasServer(id string) bool {
	_, ok := b.Servers[id]
	return ok
}

func (b *Backends) hasStaticServer(id string) bool {
	_, ok := b.StaticServers[id]
	return ok
}

func (b *Backends) hasReceiver(id string) bool {
	_, ok := b.Receivers[id]
	return ok
}

func (b *Backends) hasRadio(id string) bool {
	_, ok := b.Radios[id]
	return ok
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
