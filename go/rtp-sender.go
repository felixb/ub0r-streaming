package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/ziutek/glib"
	"github.com/ziutek/gst"
)

func (m *Manager) getRadio(config *Config) *Radio {
	id, ok := config.Servers[m.Server().Id()]
	if ok {
		return config.Backends.Radios[id]
	}
	return nil
}

func (m *Manager) setDevice(src *gst.Element, uri string) {
	if strings.Index(uri, ":") > 0 {
		parts := strings.SplitN(uri, ":", 2)
		src.SetProperty("device", parts[1])
	}
}

func (m *Manager) buildSrc(uri string) *gst.Element {
	var src *gst.Element
	if uri == "test" {
		src = makeElem("audiotestsrc")
	} else if strings.HasPrefix(uri, "alsa") {
		src = makeElem("alsasrc")
		m.setDevice(src, uri)
	} else if strings.HasPrefix(uri, "pulse") {
		src = makeElem("pulsesrc")
		m.setDevice(src, uri)
		// TODO add filter for stereo
	} else {
		src = makeElem("uridecodebin")
		src.SetProperty("uri", uri)
		src.SetProperty("buffer-duration", 1000)
	}
	return src
}

func (m *Manager) buildPipeline(uri string) {
	src := m.buildSrc(uri)
	pipe1 := makeElem("audioconvert")
	pipe2 := makeElem("audioresample")
	pipe3 := makeElem("opusenc")
	pipe3.SetProperty("bitrate", 96000)
	pipe3.SetProperty("dtx", true)
	pipe3.SetProperty("inband-fec", false)
	pipe3.SetProperty("packet-loss-percentage", 0)
	pipe4 := makeElem("oggmux")
	sink := makeElem("tcpserversink")
	s := m.Server()
	sink.SetProperty("host", s.Host)
	sink.SetProperty("port", s.Port)

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

func (m *Manager) playPipeline(uri string) {
	m.Pipeline = nil
	m.buildPipeline(uri)
	m.StartPipeline()
}

func (m *Manager) loop() {
	for true {
		log.Debug("starting new pipeline with static stream: %s", m.StaticUri)
		m.playPipeline(m.StaticUri)
		// we don't listen for new config, but errors will reset the pipeline
		m.WaitForNewConfig()
		m.StopPipeline()
	}
}

func (m *Manager) scheduleBackendTimeout(c <-chan time.Time) {
	for {
		log.Debug("ping config server")
		uri := m.ConfigUri + "/api/ping/server"
		pingConfig(uri, m.Backend)
		<-c
	}
}

func (m *Manager) startSender() {
	log.Debug("starting sender")
	go m.loop()
	go m.scheduleBackendTimeout(time.Tick(backendTimeout / 2))
	log.Debug("start gst loop")
	glib.NewMainLoop(nil).Run()
	log.Debug("sender stopped")
}

func main() {
	hostname, _ := os.Hostname()
	m := NewServer()
	flag.StringVar(&m.Server().Name, "name", hostname, "server name")
	flag.StringVar(&m.Server().Host, "host", hostname, "server host name")
	flag.IntVar(&m.Server().Port, "port", 48100, "server port")
	flag.StringVar(&m.ConfigUri, "config-server", "http://localhost:8080", "config server base uri")
	flag.StringVar(&m.StaticUri, "uri", "", "uri to stream into the network")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	if m.StaticUri == "" {
		log.Error("--uri is mandatory")
		os.Exit(1)
	}

	m.startSender()
}
