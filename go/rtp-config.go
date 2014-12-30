package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.google.com/p/go.net/websocket"
)

const (
	configCacheFile = "/tmp/rtp-config.json"
	serverTimeout   = 30 * time.Second
)

var (
	config Config
	managers       = make(map[string]*Manager)
	configCond *sync.Cond
	saveConfigLock = sync.Mutex{}
	staticDir *string
	port *int
)

// Locking -----------------------------------------

func waitForNewConfig() {
	log.Debug("Wait for configCond: %s", *configCond)
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

func (b *Backends) pingReceiver(o *Receiver) {
	id := o.Id()
	if r, ok := b.Receivers[id]; ok {
		r.Ping()
	} else {
		b.Receivers[id] = o
		o.Ping()
	}
}

func (b *Backends) pingServer(o *Server) {
	id := o.Id()
	if s, ok := b.Servers[id]; ok {
		s.Ping()
	}else {
		b.Servers[id] = o
		o.Ping()
	}
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

	for {
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
	return &o, err
}

func unmarshalServer(req *http.Request) (*Server, error) {
	var o Server
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&o)
	return &o, err
}

func unmarshalRadio(req *http.Request) (*Radio, error) {
	var o Radio
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&o)
	return &o, err
}

// POST /api/ping
func serveApiPing(w http.ResponseWriter, req *http.Request) *ServeError {
	if req.URL.Path == "/api/ping/receiver" {
		o, err := unmarshalReceiver(req)
		if err == nil {
			config.Backends.pingReceiver(o)
			return nil
		} else {
			return NewInternalError(fmt.Sprintf("somthing went wrong parsing body: %s", err))
		}
	} else if req.URL.Path == "/api/ping/server" {
		o, err := unmarshalServer(req)
		if err == nil {
			config.Backends.pingServer(o)
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
		if err != nil {
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

func findServerWithRadio(radio_id string) (string, bool) {
	for k, r := range config.Servers {
		if r == radio_id {
			return k, true
		}
	}
	return "", false
}

func findFreePort() int {
	port := 48110
	ok := false
	for !ok {
		ok = true
		// loop through servers until free port is found
		for _, s := range config.Backends.Servers {
			if s.Internal && s.Port == port {
				ok = false
				port += 1
				break
			}
		}
	}
	return port
}

func spawnServer(radio_id string) *Manager {
	r := config.Backends.Radios[radio_id]
	log.Info("spawning new sender for radio: %s", r.Uri)
	hostname, _ := os.Hostname()
	m := NewServer(true)
	s := m.Server()
	s.Name = hostname
	s.Host = hostname
	s.Port = findFreePort()
	m.ConfigUri = fmt.Sprintf("http://localhost:%d", *port)
	m.StaticUri = r.Uri
	server_id := s.Id()
	config.Backends.Servers[server_id] = s
	config.Servers[server_id] = radio_id
	return m
}

func findOrSpawnServer(radio_id string) string {
	// check if some server is already playing this stream
	server_id, ok := findServerWithRadio(radio_id)
	if ok {
		log.Debug("found running server for radio: %s, %s", radio_id, server_id)
		return server_id
	}

	// spawn new server
	m := spawnServer(radio_id)
	server_id = m.Server().Id()
	managers[server_id] = m
	go m.startSender()
	return server_id
}

func stopServer(server_id string) {
	m := managers[server_id]
	m.stopSender()
	delete(config.Servers, server_id)
	delete(config.Backends.Servers, server_id)
}

// GET /api/receiver?id=${receiver-id}&radio=${radio-id}
func serveApiReceiverRadio(w http.ResponseWriter, req *http.Request, receiver_id, radio_id string) *ServeError {
	log.Debug("/api/receiver receiver: %s, radio: %s", receiver_id, radio_id)

	if radio_id != "off" {
		if !config.Backends.hasRadio(radio_id) {
			return NewInternalError(fmt.Sprintf("server not found: %s", radio_id))
		}
	}

	log.Debug("setting new radio for %s: %s", receiver_id, radio_id)
	config.Receivers[receiver_id] = findOrSpawnServer(radio_id)
	notifyNewConfig()
	return nil
}

// GET /api/receiver?id=${receiver-id}&server=${server-id}
func serveApiReceiverServer(w http.ResponseWriter, req *http.Request, receiver_id, server_id string) *ServeError {
	log.Debug("/api/receiver receiver: %s, server: %s", receiver_id, server_id)

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

// GET /api/receiver?id=${receiver-id}&volume=[1,100]
func serveApiReceiverVolume(w http.ResponseWriter, req *http.Request, receiver_id, volume string) *ServeError {
	log.Debug("/api/receiver receiver: %s, volume: %s", receiver_id, volume)

	v, err := strconv.Atoi(volume)
	if err != nil {
		return NewInternalError(fmt.Sprintf("invalid volume '%s': %s", volume, err))
	}

	if v < 0 || v > 1000 {
		return NewInternalError(fmt.Sprintf("invalid volume '%d'", v))
	}

	log.Debug("setting new volume for %s: %d", receiver_id, v)
	config.Backends.Receivers[receiver_id].Volume = v
	notifyNewConfig()
	return nil
}

// GET /api/receiver?id=${receiver-id}&server=${server-id}
// GET /api/receiver?id=${receiver-id}&radio=${radio-id}
// GET /api/receiver?id=${receiver-id}&volume=[0,100]
func serveApiReceiver(w http.ResponseWriter, req *http.Request) *ServeError {
	receiver_id := req.URL.Query().Get("id")
	server_id := req.URL.Query().Get("server")
	radio_id := req.URL.Query().Get("radio")
	volume := req.URL.Query().Get("volume")

	if !config.Backends.hasReceiver(receiver_id) {
		return NewInternalError(fmt.Sprintf("receiver not found: %s", receiver_id))
	}

	if server_id != "" && radio_id == "" && volume == "" {
		return serveApiReceiverServer(w, req, receiver_id, server_id)
	} else if radio_id != "" && server_id == "" && volume == "" {
		return serveApiReceiverRadio(w, req, receiver_id, radio_id)
	} else if volume != "" && radio_id == "" && server_id == "" {
		return serveApiReceiverVolume(w, req, receiver_id, volume)
	} else {
		return NewInternalError("server or radio is mandatory")
	}
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
	} else if req.URL.Path == "/api/receiver" {
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
		// delete internal servers
		for k, s := range config.Backends.Servers {
			if s.Internal {
				delete(config.Backends.Servers, k)
			}
		}
		config.Servers = make(map[string]string)
		// delete receiver -> dead server
		for k, s := range config.Receivers {
			if !config.Backends.hasServer(s) {
				delete(config.Receivers, k)
			}
		}
		log.Debug("config: %s", config)
	} else {
		log.Info("create initial config")
		config.Servers = make(map[string]string)
		config.Receivers = make(map[string]string)
		config.Backends = NewBackends()
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
	for {
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
			if !o.Internal && o.LastPing < threshold {
				log.Info("remove possibly dead server: %s", o.Id())
				delete(config.Backends.Servers, k)
			}
		}
		for k := range config.Receivers {
			if !config.Backends.hasReceiver(k) {
				delete(config.Receivers, k)
			}
		}
	}
}

func scheduleServerTimeout(c <-chan time.Time) {
	for _ = range c {
		for _, s := range config.Backends.Servers {
			if !s.Internal {
				continue
			}

			// search for receivers listening to current server
			server_id := s.Id()
			found := false
			for _, v := range config.Receivers {
				if v == server_id {
					found = true
					break
				}
			}
			if !found {
				stopServer(server_id)
			}
		}
	}
}

// MAIN --------------------------------------------

func main() {
	configFile := flag.String("config-cache", configCacheFile, "File for persisting config state")
	port = flag.Int("http", 8080, "Port for binding the config server")
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
	go scheduleServerTimeout(time.Tick(serverTimeout))

	httpd(*port)

	saveConfigCache(configFile)
}
