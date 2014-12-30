package main

import (
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
		src.SetProperty("download", true)
		src.SetProperty("use-buffering", true)
		src.SetProperty("buffer-duration", 2000)
	}
	return src
}

func (m *Manager) buildPipeline(uri string) {
	src := m.buildSrc(uri)
	pipe1 := makeElem("audioconvert")
	pipe2 := makeElem("audioresample")
	pipe3 := makeElem("opusenc")
	pipe3.SetProperty("bitrate", 96000)
	pipe3.SetProperty("bandwith", "auto")
	pipe3.SetProperty("audio", true)
	pipe3.SetProperty("dtx", true)
	pipe3.SetProperty("packet-loss-percentage", 0)
	pipe4 := makeElem("oggmux")
	pipe5 := makeElem("queue2")
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
	addElem(m.Pipeline, pipe5)
	addElem(m.Pipeline, sink)
	linkElems(src, pipe1)
	linkElems(pipe1, pipe2)
	linkElems(pipe2, pipe3)
	linkElems(pipe3, pipe4)
	linkElems(pipe4, pipe5)
	linkElems(pipe5, sink)
}

func (m *Manager) playPipeline(uri string) {
	m.Pipeline = nil
	m.buildPipeline(uri)
	m.StartPipeline()
}

func (m *Manager) loop(l *glib.MainLoop) {
	for m.running {
		log.Debug("starting new pipeline with static stream: %s", m.StaticUri)
		m.playPipeline(m.StaticUri)
		// we don't listen for new config, but errors will reset the pipeline
		m.WaitForNewConfig()
		m.StopPipeline()
	}

	if l != nil {
		l.Quit()
	}
}

func (m *Manager) scheduleBackendTimeout(c <-chan time.Time) {
	for m.running {
		log.Debug("ping config server")
		uri := m.ConfigUri + "/api/ping/server"
		pingConfig(uri, m.Backend)
		<-c
	}
}

func (m *Manager) startSender() {
	log.Debug("starting sender")
	m.running = true
	l := glib.NewMainLoop(nil)
	go m.loop(l)
	if !m.Server().Internal {
		go m.scheduleBackendTimeout(time.Tick(backendTimeout / 2))
	}
	log.Debug("start gst loop")
	l.Run()
	log.Debug("sender stopped")
}

func (m *Manager) stopSender() {
	log.Info("stopping sender: %s", m.Server().Id())
	m.running = false
	m.NewConfig(nil)
}
