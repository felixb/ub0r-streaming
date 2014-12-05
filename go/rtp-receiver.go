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
	src := gst.ElementFactoryMake("tcpclientsrc", "tcpclientsrc")
	checkElem(src, "tcpclientsrc")
	src.SetProperty("host", server.Host)
	src.SetProperty("port", server.Port)

	demux := gst.ElementFactoryMake("oggdemux", "oggdemux")
	checkElem(demux, "oggdemux")

	dec := gst.ElementFactoryMake("opusdec", "opusdec")
	checkElem(dec, "opusdec")

	sink := gst.ElementFactoryMake("alsasink", "alsasink")
	checkElem(sink, "alsasink")
	sink.SetProperty("sync", false)

	m.Pipeline = gst.NewPipeline("pipeline")
	bus := m.Pipeline.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", m.onMessage, nil)
	demux.ConnectNoi("pad-added", onPadAdded, dec.GetStaticPad("sink"))

	log.Debug("added: %v", m.Pipeline.Add(src, demux, dec, sink))
	log.Debug("linked: %v", src.Link(demux))
	log.Debug("linked: %v", dec.Link(sink))
}

func playPipeline(m *Manager, server *Server) {
	m.Pipeline = nil
	if checkServer(server) {
		buildPipeline(m, server)
		m.StartPipeline()
	} else {
		// schedule recheck
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

		server := getServer(config, m.Name)
		if server != nil {
			log.Info("connectiong to server: %s:%d", server.Host, server.Port)
			playPipeline(m, server)
		} else {
			log.Info("unable to find suitable server for myself (%s), waiting for new config", m.Name)
		}
		// watch state/config changes and restart pipeline
		var newServer *Server
		for newServer == nil || (server != nil && server.Host == newServer.Host && server.Port == newServer.Port) {
			config = <-m.ConfigSync
			log.Debug("got new config: %s", config)
			if config == nil {
				// state changed start all over
				break
			}
			newServer = getServer(config, m.Name)
		}
		m.StopPipeline()
	}
}

func main() {
	initLogger()
	hostname, _ := os.Hostname()
	m := NewManager()
	flag.StringVar(&m.Name, "name", hostname, "receiver name")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.Parse()

	log.Debug("starting receiver")
	go loop(m)
	go watchConfig(m)
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}
