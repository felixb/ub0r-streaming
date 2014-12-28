package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ziutek/gst"
	"code.google.com/p/go.net/websocket"
)

const (
	retryInterval = 5 * time.Second
	maxRetry      = 24
)

// ------------ manager

type Manager struct {
	Pipeline *gst.Pipeline
	configSync chan *Config
	ConfigUri  string
	StaticUri  string
	State      gst.State
	Backend    Pinger
	RetryCount int
}

func newManager() *Manager {
	m := Manager{}
	m.configSync = make(chan *Config, 2)
	return &m
}

func NewReceiver() *Manager {
	m := newManager()
	m.Backend = &Receiver{}
	return m
}

func NewServer() *Manager {
	m := newManager()
	m.Backend = &Server{}
	return m
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
		m.NewConfig(nil)
	case gst.MESSAGE_ERROR:
		err, debug := msg.ParseError()
		log.Error("pipeline error: %s (debug: %s)", err, debug)
		// try to reconnect
		time.Sleep(retryInterval)
		m.NewConfig(nil)
	case gst.MESSAGE_BUFFERING:
		// ignore
	default:
		log.Debug("pipeline message: %s", t)
	}
}

func (m *Manager) NewConfig(config *Config) {
	m.configSync<-config
}

func (m *Manager) WaitForNewConfig() *Config {
	config := <-m.configSync
	log.Debug("got new config: %s", config)
	return config
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
	log.Debug("register object at path: %s", uri)

	j, _ := json.Marshal(obj)
	req, _ := http.NewRequest("POST", uri, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	_, err := client.Do(req)
	return err
}

func fetchObject(uri string, obj interface{}) (interface{}, error) {
	log.Debug("fetch object: %s", uri)
	resp, err := http.Get(uri)
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

func fetchConfig(base string) (*Config, error) {
	var config Config
	_, err := fetchObject(base+"/api/config", &config)
	return &config, err
}

func readBlob(ws *websocket.Conn) ([]byte, error) {
	buf := make([]byte, 0)

	for true {
		msg := make([]byte, 2048)
		var n int
		var err error
		if n, err = ws.Read(msg); err != nil {
			return nil, err
		}
		log.Debug("Received %db: %s", n, msg[:n])
		buf = append(buf, msg[:n]...)

		if len(msg) > n {
			return buf, nil
		}
	}

	// should never happen
	return nil, nil
}

func (m *Manager) readConfig(ws *websocket.Conn) error {
	buf, err := readBlob(ws)
	if err != nil {
		return err
	}

	var config Config
	err = json.Unmarshal(buf, &config)
	if err != nil {
		return err
	}

	// send new config to pipeline
	log.Debug("got new config: %s", config)
	m.NewConfig(&config)
	return nil
}

func (m *Manager) readConfigs(ws *websocket.Conn) {
	defer ws.Close()
	// read from websocket
	for true {
		err := m.readConfig(ws)
		if err != nil {
			log.Error("error reading config: %s", err)
			return
		}
	}
}

func (m *Manager) watchConfig() {
	backOff := time.Second

	for true {
		origin := m.ConfigUri
		url := strings.Replace(m.ConfigUri, "http", "ws", 1) + "/ws/config"
		ws, err := websocket.Dial(url, "", origin)
		if err != nil {
			log.Error("unable to reach config server: %s", err)
			time.Sleep(backOff)
			// exponential back off, max = 1h
			if backOff < time.Hour {
				backOff *= 2
			} else {
				backOff = time.Hour
			}
		} else {
			// reset back off
			backOff = time.Second
			m.readConfigs(ws)
		}
	}
}
