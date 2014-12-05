package main

import (
	"flag"
	"os"
	"strings"

	"github.com/ziutek/glib"
	"github.com/ziutek/gst"
)

func getRadio(config *Config, serverName string) *Radio {
	return config.Servers[serverName]
}

func findServer(backends *Backends, serverName string) *Server {
	for _, s := range backends.Servers {
		if s.Host == serverName {
			return s
		}
	}
	for _, s := range backends.StaticServers {
		if s.Host == serverName {
			return s
		}
	}
	return nil
}

func buildSrc(uri string) *gst.Element {
	var src *gst.Element
	if uri == "test" {
		src = gst.ElementFactoryMake("audiotestsrc", "audiotestsrc")
	} else if strings.HasPrefix(uri, "alsa") {
		src = gst.ElementFactoryMake("alsasrc", "alsasrc")
		if strings.HasPrefix(uri, "alsa:") {
			checkElem(src, "source")
			parts := strings.SplitN(uri, ":", 2)
			src.SetProperty("device", parts[1])
		}
	} else if strings.HasPrefix(uri, "pulse") {
		// TODO pulse mirror
	} else {
		src = gst.ElementFactoryMake("uridecodebin", "uridecodebin")
		checkElem(src, "source")
		src.SetProperty("uri", uri)
		src.SetProperty("buffer-duration", 1000)
	}
	checkElem(src, "source")
	return src
}

func buildPipeline(m *Manager, uri string, config *Server) {
	src := buildSrc(uri)
	pipe1 := gst.ElementFactoryMake("audioconvert", "audioconvert")
	checkElem(pipe1, "audioconvert")
	pipe2 := gst.ElementFactoryMake("audioresample", "audioresample")
	checkElem(pipe2, "audioresample")
	pipe3 := gst.ElementFactoryMake("opusenc", "opusenc")
	checkElem(pipe3, "opusenc")
	pipe3.SetProperty("bitrate", 96000)
	pipe3.SetProperty("dtx", true)
	pipe3.SetProperty("inband-fec", false)
	pipe3.SetProperty("packet-loss-percentage", 0)
	pipe4 := gst.ElementFactoryMake("oggmux", "oggmux")
	checkElem(pipe4, "oggmux")
	sink := gst.ElementFactoryMake("tcpserversink", "tcpserversink")
	checkElem(sink, "tcpserversink")
	sink.SetProperty("host", config.Host)
	sink.SetProperty("port", config.Port)

	m.Pipeline = gst.NewPipeline("pipeline")
	bus := m.Pipeline.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", m.onMessage, nil)
	src.ConnectNoi("pad-added", onPadAdded, pipe1.GetStaticPad("sink"))

	log.Debug("added: %v", m.Pipeline.Add(src, pipe1, pipe2, pipe3, pipe4, sink))
	log.Debug("linked: %v", src.Link(pipe1))
	log.Debug("linked: %v", pipe1.Link(pipe2))
	log.Debug("linked: %v", pipe2.Link(pipe3))
	log.Debug("linked: %v", pipe3.Link(pipe4))
	log.Debug("linked: %v", pipe4.Link(sink))
}

func playPipeline(m *Manager, uri string, config *Server) {
	m.Pipeline = nil
	buildPipeline(m, uri, config)
	m.StartPipeline()
}

func loop(m *Manager) {
	var config *Config
	var err error
	backends, err := fetchBackends(m.ConfigUri, false)
	if err != nil {
		log.Error("error fetching backend config: %s", err)
		// TODO something sane
	}
	me := findServer(backends, m.Name)
	if me == nil {
		log.Error("unable to find myself in backend config")
		// TODO something sane
	}
	for true {
		log.Debug("starting new pipeline")
		if config == nil {
			config, err = fetchConfig(m.ConfigUri, false)
			if err != nil {
				log.Error("error fetching config: %s", err)
				os.Exit(1)
			}
		}

		if m.StaticUri != "" {
			log.Info("starting static stream: %s", m.StaticUri)
			playPipeline(m, m.StaticUri, me)
			config = <-m.ConfigSync
		} else {
			radio := getRadio(config, m.Name)
			if radio != nil && radio.Uri != "off" {
				log.Info("starting radio: %s", radio.Uri)
				 playPipeline(m, radio.Uri, me)
			} else if radio != nil && radio.Uri == "off" {
				log.Info("radio turned off, waiting fo new config")
			} else {
				log.Info("unable to find suitable radio for myself (%s), waiting for new config", m.Name)
			}
			// watch state/config changes and restart pipeline
			var newRadio *Radio
			for newRadio == nil || (radio != nil && radio.Uri == newRadio.Uri) {
				config = <-m.ConfigSync
				log.Debug("got new config: %s", config)
				if config == nil {
					// state changed start all over
					break
				}
				newRadio = getRadio(config, m.Name)
				log.Debug("new radio: %s", newRadio)
			}
		}
		m.StopPipeline()
	}
}

func main() {
	initLogger()
	hostname, _ := os.Hostname()
	m := NewManager()
	flag.StringVar(&m.Name, "name", hostname, "server name")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.StringVar(&m.StaticUri, "static", "", "send a static stream")
	flag.Parse()

	log.Debug("starting receiver")
	go loop(m)
	if m.StaticUri  == "" {
		go watchConfig(m)
	}
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}
