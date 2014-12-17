package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/ziutek/glib"
	"github.com/ziutek/gst"
)

func getRadio(config *Config, serverName string) *Radio {
	return config.Servers[serverName]
}

func setDevice(src *gst.Element, uri string) {
	if strings.Index(uri, ":") > 0 {
		parts := strings.SplitN(uri, ":", 2)
		src.SetProperty("device", parts[1])
	}
}

func buildSrc(uri string) *gst.Element {
	var src *gst.Element
	if uri == "test" {
		src = makeElem("audiotestsrc")
	} else if strings.HasPrefix(uri, "alsa") {
		src = makeElem("alsasrc")
		setDevice(src, uri)
	} else if strings.HasPrefix(uri, "pulse") {
		src = makeElem("pulsesrc")
		setDevice(src, uri)
		// TODO add filter for stereo
	} else {
		src = makeElem("uridecodebin")
		src.SetProperty("uri", uri)
		src.SetProperty("buffer-duration", 1000)
	}
	return src
}

func buildPipeline(m *Manager, uri string, config *Server) {
	src := buildSrc(uri)
	pipe1 := makeElem("audioconvert")
	pipe2 := makeElem("audioresample")
	pipe3 := makeElem("opusenc")
	pipe3.SetProperty("bitrate", 96000)
	pipe3.SetProperty("dtx", true)
	pipe3.SetProperty("inband-fec", false)
	pipe3.SetProperty("packet-loss-percentage", 0)
	pipe4 := makeElem("oggmux")
	sink := makeElem("tcpserversink")
	sink.SetProperty("host", config.Host)
	sink.SetProperty("port", config.Port)

	m.Pipeline = gst.NewPipeline("pipeline")
	bus := m.Pipeline.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", m.onMessage, nil)
	src.ConnectNoi("pad-added", onPadAdded, pipe1.GetStaticPad("sink"))

	addElem(m.Pipeline, src)
	addElem(m.Pipeline, pipe1)
	addElem(m.Pipeline, pipe2)
	addElem(m.Pipeline, pipe3)
	addElem(m.Pipeline, pipe4)
	addElem(m.Pipeline, sink)
	linkElems(src, pipe1)
	linkElems(pipe1, pipe2)
	linkElems(pipe2, pipe3)
	linkElems(pipe3, pipe4)
	linkElems(pipe4, sink)
}

func playPipeline(m *Manager, uri string, config *Server) {
	m.Pipeline = nil
	buildPipeline(m, uri, config)
	m.StartPipeline()
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

		if m.StaticUri != "" {
			log.Info("starting static stream: %s", m.StaticUri)
			playPipeline(m, m.StaticUri, m.Server())
			config = <-m.ConfigSync
		} else {
			radio := getRadio(config, m.Server().Host)
			if radio != nil && radio.Uri != "off" {
				log.Info("starting radio: %s", radio.Uri)
				playPipeline(m, radio.Uri, m.Server())
			} else if radio != nil && radio.Uri == "off" {
				log.Info("radio turned off, waiting fo new config")
			} else {
				log.Info("unable to find suitable radio for myself (%s), waiting for new config", m.Server().Host)
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
				newRadio = getRadio(config, m.Server().Host)
				log.Debug("new radio: %s", newRadio)
			}
		}
		m.StopPipeline()
	}
}

func (m *Manager) scheduleBackendTimeout(c <-chan time.Time) {
	for {
		log.Debug("ping config server")
		uri := m.ConfigUri + "/api/ping/"
		if m.StaticUri == "" {
			pingConfig(uri+"server", m.Backend)
		} else {
			pingConfig(uri+"static", m.Backend)
		}
		<-c
	}
}

func main() {
	hostname, _ := os.Hostname()
	m := NewManager()
	m.Backend = &Server{}
	flag.StringVar(&m.Server().Name, "name", hostname, "server name")
	flag.StringVar(&m.Server().Host, "host", hostname, "server host name")
	flag.IntVar(&m.Server().Port, "port", 48100, "server port")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.StringVar(&m.StaticUri, "static", "", "send a static stream")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)


	log.Debug("starting receiver")
	go loop(m)
	if m.StaticUri == "" {
		go watchConfig(m)
	}
	go m.scheduleBackendTimeout(time.Tick(backendTimeout / 2))
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("receiver stopped")
}
