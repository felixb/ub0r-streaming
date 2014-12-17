package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/ziutek/gst"
)

const (
	retryInterval = 5 * time.Second
	maxRetry      = 24
)

// ------------ manager

type Manager struct {
	Pipeline *gst.Pipeline
	ConfigSync chan *Config
	ConfigUri  string
	StaticUri  string
	State      gst.State
	Backend    Pinger
	RetryCount int
}

func NewManager() *Manager {
	m := Manager{}
	m.ConfigSync = make(chan *Config, 2)
	return &m
}

func (m *Manager) Receiver() *Receiver {
	return m.Backend.(*Receiver)
}

func (m *Manager) Server() *Server {
	return m.Backend.(*Server)
}

func (m *Manager) onMessage(bus *gst.Bus, msg *gst.Message) {
	t := msg.GetType()
	switch t {
	case gst.MESSAGE_STATE_CHANGED:
		pl := m.Pipeline
		if pl != nil {
			s, _, _ := pl.GetState(100)
			if s != m.State {
				log.Info("pipeline state: %s", s)
				m.State = s
			}
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
		log.Debug("pipeline message: %s", t)
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

func checkElem(e interface{}, name string) {
	if e == nil {
		log.Fatal("can't make element: %s", name)
		os.Exit(1) // TODO don't exit
	}
}

func makeElem(name string) *gst.Element {
	e := gst.ElementFactoryMake(name, name)
	checkElem(e, name)
	return e
}

func addElem(pl *gst.Pipeline, e *gst.Element) bool {
	r := pl.Add(e)
	log.Debug("add %s to pipeline: %v", e.GetName(), r)
	return r
}

func linkElems(src, sink *gst.Element) bool {
	r := src.Link(sink)
	log.Debug("link %s -> %s: %v", src.GetName(), sink.GetName(), r)
	return r
}

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

// ------------ config stuff

func pingConfig(uri string, obj Pinger) error {
	log.Debug("register object at path: %s %s", uri)

	j, _ := json.Marshal(obj)
	req, _ := http.NewRequest("POST", uri, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	_, err := client.Do(req)
	return err
}

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
