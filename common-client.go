package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/ziutek/gst"
)

func checkElem(e interface{}, name string) {
	if e == nil {
		log.Fatal("can't make element: %s", name)
		os.Exit(1) // TODO don't exit
	}
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

func watchConfig(serverUri, receiverName string, c chan *Config) {
	for true {
		config, _ := fetchConfig(serverUri, true)
		log.Debug("got new config: %s", config)
		if config != nil {
			// send new config to pipeline
			c<-config
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}
