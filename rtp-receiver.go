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

const (
	retryInterval = 10 * time.Second
)

var (
	configChan = make(chan *Config, 2)
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

func onMessage(bus *gst.Bus, msg *gst.Message) {
	log.Info("message: %s", msg.GetType())
	switch msg.GetType() {
	case gst.MESSAGE_EOS:
		configChan<-nil
	case gst.MESSAGE_ERROR:
		err, debug := msg.ParseError()
		log.Error("Error: %s (debug: %s)", err, debug)
		// try to reconnect
		time.Sleep(retryInterval)
		configChan<-nil
	}
}

func buildPipeline(server *Server) *gst.Pipeline {
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

	pl := gst.NewPipeline("pipeline")
	bus := pl.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", onMessage, nil)

	log.Debug("added: %v", pl.Add(src, demux, dec, sink))
	log.Debug("linked: %v", src.Link(demux))
	demux.ConnectNoi("pad-added", onPadAdded, dec.GetStaticPad("sink"))
	log.Debug("linked: %v", dec.Link(sink))

	return pl
}

func playPipeline(server *Server) *gst.Pipeline {
	if checkServer(server) {
		pl := buildPipeline(server)
		pl.SetState(gst.STATE_PLAYING)
		return pl
	} else {
		// schedule recheck
		time.Sleep(retryInterval)
		configChan<-nil
		return nil
	}
}

func loop(configUri, receiverName string) {
	var config *Config
	var err error
	for true {
		log.Debug("starting new pipeline")
		if config == nil {
			config, err = fetchConfig(configUri, false)
			if err != nil {
				log.Error("error fetching config: %s", err)
				os.Exit(1)
			}
		}

		var pl *gst.Pipeline
		server := getServer(config, receiverName)
		if server != nil {
			log.Info("connectiong to server: %s:%d", server.Host, server.Port)
			pl = playPipeline(server)
		} else {
			log.Info("unable to find suitable server for myself (%s), waiting for new config", receiverName)
		}
		// watch state/config changes and restart pipeline
		var newServer *Server
		for newServer == nil || (server != nil && server.Host == newServer.Host && server.Port == newServer.Port) {
			config = <-configChan
			log.Debug("got new config: %s", config)
			if config == nil {
				// state changed start all over
				break
			}
			newServer = getServer(config, receiverName)
		}
		if pl != nil {
			pl.SetState(gst.STATE_NULL)
			pl.Unref()
		}
	}
}

func main() {
	initLogger()
	hostname, _ := os.Hostname()
	name := flag.String("name", hostname, "receiver name")
	configUri := flag.String("config-server", "http://localhost:8080", "config server base uri")
	flag.Parse()

	log.Debug("starting receiver")
	go loop(*configUri, *name)
	go watchConfig(*configUri, *name, configChan)
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}
