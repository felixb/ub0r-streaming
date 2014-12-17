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

func getServer(config *Config, receiverName string) *Server {
	return config.Receivers[receiverName]
}

func checkServer(server *Server) bool {
	conn, err := net.Dial("tcp", net.JoinHostPort(server.Host, strconv.Itoa(server.Port)))
	if conn != nil {
		conn.Close()
	}
	if err != nil {
		log.Error("error connecting to server: %s", err)
	}
	return err == nil
}

func buildPipeline(m *Manager, server *Server) {
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

func playPipeline(m *Manager, server *Server) {
	m.Pipeline = nil
	if checkServer(server) {
		m.RetryCount = 0
		buildPipeline(m, server)
		m.StartPipeline()
	} else if m.RetryCount >= maxRetry {
		log.Warning("max retries reached, wait for new config")
	} else {
		// schedule recheck
		m.RetryCount += 1
		time.Sleep(retryInterval)
		m.ConfigSync<-nil
	}
}

func loop(m *Manager) {
	var config *Config
	var err error
	for true {
		log.Debug("starting new pipeline")
		if config == nil {
			config, err = fetchConfig(m.ConfigUri, false)
			if err != nil {
				log.Error("error fetching config: %s", err)
				os.Exit(1)
			}
		}

		server := getServer(config, m.Receiver().Host)
		if server != nil {
			log.Info("connecting to server: %s:%d", server.Host, server.Port)
			playPipeline(m, server)
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
			config = <-m.ConfigSync
			log.Debug("got new config: %s", config)
			if config == nil {
				// state changed start all over
				break
			}
			newServer = getServer(config, m.Receiver().Host)
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
		uri := m.ConfigUri+"/api/ping/"
		pingConfig(uri + "receiver", m.Backend)
		<-c
	}
}

func main() {
	hostname, _ := os.Hostname()
	m := NewManager()
	m.Backend = &Receiver{}
	flag.StringVar(&m.Receiver().Name, "name", hostname, "receiver name")
	flag.StringVar(&m.Receiver().Host, "host", hostname, "receiver host name")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	log.Debug("starting receiver")
	go loop(m)
	go watchConfig(m)
	go m.scheduleBackendTimeout(time.Tick(backendTimeout / 2))
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}
