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

func addReceiver(o *Receiver) {
	log.Debug("receiver: %s", *o)
	for i, oo := range config.Backends.Receivers {
		if oo.Host == o.Host {
			config.Backends.Receivers[i] = o
			return
		}
	}
	config.Backends.Receivers = append(config.Backends.Receivers, o)
}

func addServer(o *Server) {
	for i, oo := range config.Backends.Servers {
		if oo.Host == o.Host {
			config.Backends.Servers[i] = o
			return
		}
	}
	config.Backends.Servers = append(config.Backends.Servers, o)
}

func addStatic(o *Server) {
	for i, oo := range config.Backends.StaticServers {
		if oo.Host == o.Host {
			config.Backends.StaticServers[i] = o
			return
		}
	}
	config.Backends.StaticServers = append(config.Backends.StaticServers, o)
}

func addRadio(id string, o *Radio) {
	for i, oo := range config.Backends.Radios {
		if oo.Id() == id {
			config.Backends.Radios[i] = o
			return
		}
	}
	config.Backends.Radios = append(config.Backends.Radios, o)
}

func rmRadio(id string) bool {
	for i, oo := range config.Backends.Radios {
		if oo.Id() == id {
			config.Backends.Radios = append(config.Backends.Radios[:i], config.Backends.Radios[i+1:]...)
			return true
		}
	}
	return false
}

// POST /api/ping
func serveApiPing(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/api/ping/receiver" {
		o, err := unmarshalReceiver(req)
		if o != nil && err == nil {
			addReceiver(o)
			o.Ping()
		} else {
			log.Error("somthing went wrong parsing body: %s", err)
		}
	} else if req.URL.Path == "/api/ping/server" {
		o, err := unmarshalServer(req)
		if o != nil && err == nil {
			addServer(o)
			o.Ping()
		} else {
			log.Error("somthing went wrong parsing body: %s", err)
		}
	} else if req.URL.Path == "/api/ping/static" {
		o, err := unmarshalServer(req)
		if o != nil && err == nil {
			addStatic(o)
			o.Ping()
		} else {
			log.Error("somthing went wrong parsing body: %s", err)
		}
	} else {
		log.Error("unknown path: %s", req.URL.Path)
	}
}

// POST /api/radio
func serveApiRadio(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	log.Debug("/api/radio id: %s", id)
	if req.Method == "POST" {
		o, err := unmarshalRadio(req)
		if o == nil || err != nil {
			msg := fmt.Sprintf("somthing went wrong parsing body: %s", err)
			log.Error(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		addRadio(id, o)
		notifyNewConfig()
		serveJson(w, req, o)
	} else if req.Method == "DELETE" {
		if rmRadio(id) {
			notifyNewConfig()
			serveJson(w, req, nil)
		} else {
			http.NotFound(w, req)
		}
	} else {
		msg := fmt.Sprintf("unknown path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

// GET /api/server/${server-name}/radio/${radio-id}
func serveApiServer(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 6 {
		msg := fmt.Sprintf("invalid path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	serverName := parts[3]
	radioId, err := strconv.Atoi(parts[5])
	var radio *Radio
	if err != nil {
		msg := fmt.Sprintf("error parsing radio id: %s", parts[2])
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if parts[4] != "radio" {
		msg := fmt.Sprintf("invalid path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	if radioId < 0 || radioId >= len(config.Backends.Radios) {
		msg := fmt.Sprintf("invalid radio id: %d", radioId)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	radio = config.Backends.Radios[radioId]

	for i := range config.Backends.Servers {
		if config.Backends.Servers[i].Host == serverName {
			log.Debug("setting new radio for %s: %s", serverName, radio)
			config.Servers[serverName] = radio
			notifyNewConfig()
			return
		}
	}

	msg := fmt.Sprintf("receiver not found: %s", serverName)
	log.Error(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

// GET /api/receiver/${receiver-name}/{server,static}/${server-id}
func serveApiReceiver(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 6 {
		msg := fmt.Sprintf("invalid path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	receiverName := parts[3]
	serverId, err := strconv.Atoi(parts[5])
	var server *Server
	if err != nil {
		msg := fmt.Sprintf("error parsing server id: %s", parts[2])
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if parts[4] == "server" {
		if serverId < 0 || serverId >= len(config.Backends.Servers) {
			msg := fmt.Sprintf("invalid server id: %d", serverId)
			log.Error(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		server = config.Backends.Servers[serverId]
	} else if parts[4] == "static" {
		if serverId < 0 || serverId >= len(config.Backends.StaticServers) {
			msg := fmt.Sprintf("invalid server id: %d", serverId)
			log.Error(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		server = config.Backends.StaticServers[serverId]
	} else if parts[4] == "off" {
		server = nil
	} else {
		msg := fmt.Sprintf("invalid path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	for i := range config.Backends.Receivers {
		r := *config.Backends.Receivers[i]
		if r.Host == receiverName {
			log.Debug("setting new server for %s: %s", receiverName, server)
			config.Receivers[receiverName] = server
			notifyNewConfig()
			return
		}
	}

	msg := fmt.Sprintf("receiver not found: %s", receiverName)
	log.Error(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func serveJson(w http.ResponseWriter, req *http.Request, obj interface{}) {
	b, err := json.Marshal(obj)
	if err != nil {
		msg := fmt.Sprintf("error writing json: %v", err)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
	} else {
		w.Header().Add("Content-Type", "application/json")
		w.Write(b)
	}
}

func serve(w http.ResponseWriter, req *http.Request) {
	log.Debug("serve: %s %s", req.Method, req.URL.Path)

	if req.URL.Path == "/" {
		localPath := *staticDir + "/index.html"
		http.ServeFile(w, req, localPath)
	} else if req.Method == "POST" && strings.HasPrefix(req.URL.Path, "/api/ping") {
		serveApiPing(w, req)
	} else if (req.Method == "POST" || req.Method == "DELETE") && req.URL.Path == "/api/radio" {
		serveApiRadio(w, req)
	} else if req.URL.Path == "/api/config" {
		serveJson(w, req, config)
	} else if strings.HasPrefix(req.URL.Path, "/api/server/") {
		serveApiServer(w, req)
	} else if strings.HasPrefix(req.URL.Path, "/api/receiver/") {
		serveApiReceiver(w, req)
	} else {
		http.NotFound(w, req)
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
		config.Servers = make(map[string]*Radio)
		config.Receivers = make(map[string]*Server)
		config.Backends = &Backends{}
		config.Backends.Radios = []*Radio{&Radio{"Off", "off"}, &Radio{"Test", "test"}}
		config.Backends.Servers = []*Server{}
		config.Backends.StaticServers = []*Server{}
		config.Backends.Receivers = []*Receiver{}
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

		for i := len(config.Backends.Receivers) - 1; i >= 0 ; i-- {
			o := config.Backends.Receivers[i]
			if o.LastPing < threshold {
				log.Info("remove possibly dead receiver: %s", o.Host)
				config.Backends.Receivers = append(config.Backends.Receivers[:i], config.Backends.Receivers[i+1:]...)
			}
		}

		for i := len(config.Backends.Servers) - 1; i >= 0 ; i-- {
			o := config.Backends.Servers[i]
			if o.LastPing < threshold {
				log.Info("remove possibly dead server: %s", o.Host)
				config.Backends.Servers = append(config.Backends.Servers[:i], config.Backends.Servers[i+1:]...)
			}
		}

		for i := len(config.Backends.StaticServers) - 1; i >= 0 ; i-- {
			o := config.Backends.StaticServers[i]
			if o.LastPing < threshold {
				log.Info("remove possibly dead static server: %s", o.Host)
				config.Backends.StaticServers = append(config.Backends.StaticServers[:i], config.Backends.StaticServers[i+1:]...)
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
