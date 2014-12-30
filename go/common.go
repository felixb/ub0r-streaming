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
	Internal bool
	LastPing int64
	RadioId  string
	RadioUri string
}

type Receiver struct {
	Name     string
	Host     string
	LastPing int64
	Volume   int
	ServerId string
}

type Config struct {
	Radios        map[string]*Radio
	Receivers     map[string]*Receiver
	Servers       map[string]*Server
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
	return fmt.Sprintf("receiver-%s", r.Name)
}

func (r *Radio) Id() string {
	return fmt.Sprintf("radio-%x", sha1.Sum([]byte(r.Uri)))
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
