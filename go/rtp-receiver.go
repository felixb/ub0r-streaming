package main

import (
	"flag"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ziutek/glib"
	"github.com/ziutek/gst"
)

func (m *Manager) getServer(config *Config) *Server {
	id, ok := config.Receivers[m.Receiver().Id()]
	if ok {
		s, ok := config.Backends.Servers[id]
		if ok {
			return s
		} else {
			return config.Backends.StaticServers[id]
		}
	}
	return nil
}

func (m *Manager) checkServer(server *Server) bool {
	conn, err := net.Dial("tcp", net.JoinHostPort(server.Host, strconv.Itoa(server.Port)))
	if conn != nil {
		conn.Close()
	}
	if err != nil {
		log.Error("error connecting to server: %s", err)
	}
	return err == nil
}

func (m *Manager) buildPipeline(server *Server) {
	src := makeElem("tcpclientsrc")
	src.SetProperty("host", server.Host)
	src.SetProperty("port", server.Port)
	demux := makeElem("oggdemux")
	dec := makeElem("opusdec")
	sink := makeElem("alsasink")
	sink.SetProperty("sync", false)

	m.Pipeline = gst.NewPipeline("pipeline")
	bus := m.Pipeline.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", m.onMessage, nil)
	demux.ConnectNoi("pad-added", onPadAdded, dec.GetStaticPad("sink"))

	addElem(m.Pipeline, src)
	addElem(m.Pipeline, demux)
	addElem(m.Pipeline, dec)
	addElem(m.Pipeline, sink)
	linkElems(src, demux)
	linkElems(dec, sink)
}

func (m *Manager) playPipeline(server *Server) {
	m.Pipeline = nil
	if m.checkServer(server) {
		m.RetryCount = 0
		m.buildPipeline(server)
		m.StartPipeline()
	} else if m.RetryCount >= maxRetry {
		log.Warning("max retries reached, wait for new config")
	} else {
		// schedule recheck
		m.RetryCount += 1
		time.Sleep(retryInterval)
		m.NewConfig(nil)
		m.RetryCount = 0
	}
}

func (m *Manager) loop() {
	var config *Config
	var err error
	for true {
		log.Debug("starting new pipeline")
		if config == nil {
			config, err = fetchConfig(m.ConfigUri)
			if err != nil {
				log.Error("error fetching config: %s", err)
				os.Exit(1)
			}
		}

		server := m.getServer(config)
		if server != nil {
			log.Info("connecting to server: %s:%d", server.Host, server.Port)
			m.playPipeline(server)
		} else {
			log.Info("unable to find suitable server for myself (%s), waiting for new config", m.Receiver().Host)
		}
		// watch state/config changes and restart pipeline
		var newServer *Server
		i := 0
		for newServer == nil || (server != nil && server.Host == newServer.Host && server.Port == newServer.Port) {
			log.Debug("wait for new config")
			log.Debug("old server: %s", server)
			log.Debug("new server: %s", newServer)
			config = m.WaitForNewConfig()
			if config == nil {
				// state changed start all over
				break
			}
			newServer = m.getServer(config)
			// exit loop if server == off
			if newServer == nil {
				if i > 0 {
					time.Sleep(retryInterval)
				}
				break
			}
			i += 1
		}
		m.StopPipeline()
	}
}

func (m *Manager) scheduleBackendTimeout(c <-chan time.Time) {
	for {
		log.Debug("ping config server")
		uri := m.ConfigUri + "/api/ping/"
		pingConfig(uri+"receiver", m.Backend)
		<-c
	}
}

func (m *Manager) startReceiver() {
	log.Debug("starting receiver")
	go m.loop()
	go m.watchConfig()
	go m.scheduleBackendTimeout(time.Tick(backendTimeout / 2))
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}

func main() {
	hostname, _ := os.Hostname()
	m := NewReceiver()
	flag.StringVar(&m.Receiver().Name, "name", hostname, "receiver name")
	flag.StringVar(&m.Receiver().Host, "host", hostname, "receiver host name")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	m.startReceiver()
}
