package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/ziutek/gst"
)

const (
	retryInterval = 10 * time.Second
)

// ------------ manager

type Manager struct {
	Pipeline *gst.Pipeline
	ConfigSync chan *Config
	ConfigUri  string
	StaticUri  string
	State      gst.State
	Name       string
}

func NewManager() *Manager {
	m := Manager{}
	m.ConfigSync = make(chan *Config, 2)
	return &m
}

func (m *Manager) onMessage(bus *gst.Bus, msg *gst.Message) {
	t := msg.GetType()
	switch t {
	case gst.MESSAGE_STATE_CHANGED:
		s, _, _ := m.Pipeline.GetState(100)
		if s != m.State {
			log.Info("pipeline state: %s", s)
			m.State = s
		}
	case gst.MESSAGE_EOS:
		log.Info("pipeline: end of stream")
		m.ConfigSync<-nil
	case gst.MESSAGE_ERROR:
		err, debug := msg.ParseError()
		log.Error("pipeline error: %s (debug: %s)", err, debug)
		// try to reconnect
		time.Sleep(retryInterval)
		m.ConfigSync<-nil
	case gst.MESSAGE_BUFFERING:
		// ignore
	default:
		log.Info("pipeline message: %s", t)
	}
}

func (m *Manager) StartPipeline() {
	if m.Pipeline != nil {
		log.Info("start pipeline")
		m.Pipeline.SetState(gst.STATE_PLAYING)
	}
}

func (m *Manager) StopPipeline() {
	if m.Pipeline != nil {
		log.Info("stop pipeline")
		m.Pipeline.SetState(gst.STATE_NULL)
		m.Pipeline.Unref()
		m.Pipeline = nil
	}
}

// ------------ gst stuff

func onPadAdded(sinkPad, newPad *gst.Pad) {
	log.Debug("pad-added: %s", newPad.GetName())
	log.Debug("sink pad: %s", sinkPad.GetName())
	if newPad.CanLink(sinkPad) {
		if newPad.Link(sinkPad) != gst.PAD_LINK_OK {
			log.Error("error linking pads: %s/%s", newPad.GetName(), sinkPad.GetName())
		}
	} else {
		log.Error("unable to link pads: %s/%s", newPad.GetName(), sinkPad.GetName())
	}
}

func checkElem(e interface{}, name string) {
	if e == nil {
		log.Fatal("can't make element: %s", name)
		os.Exit(1) // TODO don't exit
	}
}

// ------------ config stuff

func fetchObject(uri string, wait bool, obj interface{}) (interface{}, error) {
	var u string
	if wait {
		u = uri+"?wait"
	} else {
		u = uri
	}
	log.Debug("fetch object: %s", u)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func fetchBackends(base string, wait bool) (*Backends, error) {
	var backends Backends
	_, err := fetchObject(base+"/api/backends", wait, &backends)
	return &backends, err
}

func fetchConfig(base string, wait bool) (*Config, error) {
	var config Config
	_, err := fetchObject(base+"/api/config", wait, &config)
	return &config, err
}

func watchConfig(m *Manager) {
	for true {
		config, _ := fetchConfig(m.ConfigUri, true)
		log.Debug("got new config: %s", config)
		if config != nil {
			// send new config to pipeline
			m.ConfigSync<-config
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}
