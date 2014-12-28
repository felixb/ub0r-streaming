package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"code.google.com/p/go.net/websocket"
)

const (
	configCacheFile = "/tmp/rtp-config.json"
)

var (
	config Config
	configCond *sync.Cond
	saveConfigLock = sync.Mutex{}
	staticDir *string
)

// Locking -----------------------------------------

func waitForNewConfig() {
	log.Debug("Wait for configCond: %s", configCond)
	configCond.L.Lock()
	configCond.Wait()
	configCond.L.Unlock()
}

func notifyNewConfig() {
	configCond.Broadcast()
}

// Errors ------------------------------------------

type ServeError struct {
	Message      string
	ResponseCode int
}

func NewError(msg string, rc int) *ServeError {
	return &ServeError{msg, rc}
}

func NewInternalError(msg string) *ServeError {
	return &ServeError{msg, http.StatusInternalServerError}
}

func (e *ServeError) Error() string {
	return e.Message
}

// Interfaces --------------------------------------

func NewBackends() *Backends {
	b := &Backends{}
	b.Radios = make(map[string]*Radio)
	b.Servers = make(map[string]*Server)
	b.Receivers = make(map[string]*Receiver)
	return b
}

func (b *Backends) addReceiver(o *Receiver) {
	b.Receivers[o.Id()] = o
}

func (b *Backends) addServer(o *Server) {
	b.Servers[o.Id()] = o
}

func (b *Backends) addRadio(o *Radio) {
	b.Radios[o.Id()] = o
}

func (b *Backends) rmRadio(id string) bool {
	if _, ok := b.Radios[id]; ok {
		delete(b.Radios, id)
		return true
	} else {
		return false
	}
}

// HTTP --------------------------------------------

// WebSocket /ws/config
func serveWsConfig(ws *websocket.Conn) {
	log.Info("serve: /ws/config")

	for true {
		waitForNewConfig()
		b, err := json.Marshal(config)
		if err != nil {
			msg := fmt.Sprintf("error writing json: %v", err)
			log.Error(msg)
		} else {
			ws.Write(b)
		}
	}
}

func unmarshalReceiver(req *http.Request) (*Receiver, error) {
	var o Receiver
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func unmarshalServer(req *http.Request) (*Server, error) {
	var o Server
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func unmarshalRadio(req *http.Request) (*Radio, error) {
	var o Radio
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// POST /api/ping
func serveApiPing(w http.ResponseWriter, req *http.Request) *ServeError {
	if req.URL.Path == "/api/ping/receiver" {
		o, err := unmarshalReceiver(req)
		if o != nil && err == nil {
			config.Backends.addReceiver(o)
			o.Ping()
			return nil
		} else {
			return NewInternalError(fmt.Sprintf("somthing went wrong parsing body: %s", err))
		}
	} else if req.URL.Path == "/api/ping/server" {
		o, err := unmarshalServer(req)
		if o != nil && err == nil {
			config.Backends.addServer(o)
			o.Ping()
			return nil
		} else {
			return NewInternalError(fmt.Sprintf("somthing went wrong parsing body: %s", err))
		}
	} else {
		return NewInternalError(fmt.Sprintf("unknown path: %s", req.URL.Path))
	}
}

// POST /api/radio
func serveApiRadio(w http.ResponseWriter, req *http.Request) *ServeError {
	id := req.URL.Query().Get("id")
	log.Debug("/api/radio id: %s", id)
	if req.Method == "POST" {
		o, err := unmarshalRadio(req)
		if o == nil || err != nil {
			return NewInternalError(fmt.Sprintf("somthing went wrong parsing body: %s", err))
		}
		config.Backends.addRadio(o)
		notifyNewConfig()
		serveJson(w, req, o)
	} else if req.Method == "DELETE" {
		if config.Backends.rmRadio(id) {
			notifyNewConfig()
			serveJson(w, req, nil)
		} else {
			return NewError("radio not found", http.StatusNotFound)
		}
	} else {
		return NewInternalError(fmt.Sprintf("unknown path: %s", req.URL.Path))
	}
	return nil
}

// GET /api/server?server=${server-id}&radio=${radio-id}
func serveApiServer(w http.ResponseWriter, req *http.Request) *ServeError {
	server_id := req.URL.Query().Get("server")
	radio_id := req.URL.Query().Get("radio")
	log.Debug("/api/server server: %s, radio: %s", server_id, radio_id)

	if !config.Backends.hasServer(server_id) {
		return NewInternalError(fmt.Sprintf("server not found: %s", server_id))
	}

	if !config.Backends.hasRadio(radio_id) {
		return NewInternalError(fmt.Sprintf("radio not found: %s", radio_id))
	}

	log.Debug("setting new radio for %s: %s", server_id, radio_id)
	config.Servers[server_id] = radio_id
	notifyNewConfig()
	return nil
}

// GET /api/receiver?receiver=${receiver-id}&server=${server-id}
func serveApiReceiver(w http.ResponseWriter, req *http.Request) *ServeError {
	receiver_id := req.URL.Query().Get("receiver")
	server_id := req.URL.Query().Get("server")
	log.Debug("/api/receiver receiver: %s, server: %s", receiver_id, server_id)

	if !config.Backends.hasReceiver(receiver_id) {
		return NewInternalError(fmt.Sprintf("receiver not found: %s", receiver_id))
	}

	if server_id != "off" {
		if !config.Backends.hasServer(server_id) {
			return NewInternalError(fmt.Sprintf("server not found: %s", server_id))
		}
	}

	log.Debug("setting new server for %s: %s", receiver_id, server_id)
	config.Receivers[receiver_id] = server_id
	notifyNewConfig()
	return nil
}

func serveJson(w http.ResponseWriter, req *http.Request, obj interface{}) *ServeError {
	b, err := json.Marshal(obj)
	if err != nil {
		return NewInternalError(fmt.Sprintf("error writing json: %v", err))
	} else {
		w.Header().Add("Content-Type", "application/json")
		w.Write(b)
		return nil
	}
}

func serve(w http.ResponseWriter, req *http.Request) {
	log.Debug("serve: %s %s", req.Method, req.URL.Path)

	var err *ServeError
	if req.URL.Path == "/" {
		localPath := *staticDir + "/index.html"
		http.ServeFile(w, req, localPath)
	} else if req.Method == "POST" && strings.HasPrefix(req.URL.Path, "/api/ping") {
		err = serveApiPing(w, req)
	} else if (req.Method == "POST" || req.Method == "DELETE") && req.URL.Path == "/api/radio" {
		err = serveApiRadio(w, req)
	} else if req.URL.Path == "/api/config" {
		err = serveJson(w, req, config)
	} else if req.URL.Path == "/api/server/" {
		err = serveApiServer(w, req)
	} else if req.URL.Path == "/api/receiver/" {
		err = serveApiReceiver(w, req)
	} else {
		http.NotFound(w, req)
	}

	if err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func httpd(port int) {
	log.Info("starting httpd on port %d", port)
	addr := fmt.Sprintf(":%d", port)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(*staticDir))))
	http.Handle("/ws/config", websocket.Handler(serveWsConfig))
	http.Handle("/", http.HandlerFunc(serve))
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Error("error starting httpd: %v", err)
	}
}

// INIT --------------------------------------------

func loadConfigCache(configFile *string) {
	if _, err := os.Stat(*configFile); err == nil {
		log.Info("loading config cache file %s", *configFile)
		d, err := ioutil.ReadFile(*configFile)
		if err != nil {
			log.Error("error reading config: %v", err)
			return
		}
		json.Unmarshal(d, &config)
		log.Debug("config: %s", config)
	} else {
		log.Info("create initial config")
		config.Servers = make(map[string]string)
		config.Receivers = make(map[string]string)
		config.Backends = NewBackends()
		config.Backends.addRadio(&Radio{"Off", "off"})
		config.Backends.addRadio(&Radio{"Test", "test"})
	}
}

func saveConfigCache(configFile *string) {
	d, err := json.Marshal(&config)
	if err != nil {
		log.Error("error writing config: %v", err)
	}
	saveConfigLock.Lock()
	err = ioutil.WriteFile(*configFile, d, 0644)
	saveConfigLock.Unlock()
	if err != nil {
		log.Error("error writing config: %v", err)
	} else {
		log.Debug("wrote state to: %s", *configFile)
	}
}

func scheduleSaveConfigCache(configFile *string) {
	for true {
		waitForNewConfig()
		saveConfigCache(configFile)
	}
}

func scheduleBackendTimeout(c <-chan time.Time) {
	for t := range c {
		now := t.Unix()
		threshold := now - int64(backendTimeout / time.Second)

		for k, o := range config.Backends.Receivers {
			if o.LastPing < threshold {
				log.Info("remove possibly dead receiver: %s", o.Id())
				delete(config.Backends.Receivers, k)
			}
		}
		for k, o := range config.Backends.Servers {
			if o.LastPing < threshold {
				log.Info("remove possibly dead server: %s", o.Id())
				delete(config.Backends.Servers, k)
			}
		}
	}
}

// MAIN --------------------------------------------

func main() {
	configFile := flag.String("config-cache", configCacheFile, "File for persisting config state")
	port := flag.Int("http", 8080, "Port for binding the config server")
	staticDir = flag.String("webroot", "static", "Directory for serving static content")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)

	log.Info("starting")
	locker := &sync.Mutex{}
	configCond = sync.NewCond(locker)

	loadConfigCache(configFile)
	go scheduleSaveConfigCache(configFile)
	go scheduleBackendTimeout(time.Tick(backendTimeout))

	httpd(*port)

	saveConfigCache(configFile)
}
