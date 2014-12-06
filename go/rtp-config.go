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

	"code.google.com/p/go.net/websocket"
	"gopkg.in/yaml.v2"
)

const (
	configCacheFile = "/tmp/rtp-config.yaml"
)

var (
	backends Backends
	config Config
	configCond *sync.Cond
	saveConfigLock = sync.Mutex{}
	staticDir *string
)

// HTTP --------------------------------------------

// /ws/config
func serveWsConfig(ws *websocket.Conn) {
	log.Info("serve: /ws/config")

	for true {
		log.Debug("Wait for configCond: %s", configCond)
		configCond.L.Lock()
		configCond.Wait()
		configCond.L.Unlock()

		b, err := json.Marshal(config)
		if err != nil {
			msg := fmt.Sprintf("error writing json: %v", err)
			log.Error(msg)
		} else {
			ws.Write(b)
		}
	}
}

// /api/server/${server-name}/radio/${radio-id}
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

	if radioId < 0 || radioId >= len(backends.Radios) {
		msg := fmt.Sprintf("invalid radio id: %d", radioId)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	radio = backends.Radios[radioId]

	for i := range backends.Servers {
		if backends.Servers[i].Host == serverName {
			log.Debug("setting new radio for %s: %s", serverName, radio)
			config.Servers[serverName] = radio
			configCond.Broadcast()
			return
		}
	}

	msg := fmt.Sprintf("receiver not found: %s", serverName)
	log.Error(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

// /api/receiver/${receiver-name}/{server,static}/${server-id}
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
		if serverId < 0 || serverId >= len(backends.Servers) {
			msg := fmt.Sprintf("invalid server id: %d", serverId)
			log.Error(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		server = backends.Servers[serverId]
	} else if parts[4] == "static" {
		if serverId < 0 || serverId >= len(backends.StaticServers) {
			msg := fmt.Sprintf("invalid server id: %d", serverId)
			log.Error(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		server = backends.StaticServers[serverId]
	} else if parts[4] == "off" {
		server = nil
	} else {
		msg := fmt.Sprintf("invalid path: %s", req.URL.Path)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	for i := range backends.Receivers {
		if *backends.Receivers[i] == receiverName {
			log.Debug("setting new server for %s: %s", receiverName, server)
			config.Receivers[receiverName] = server
			configCond.Broadcast()
			return
		}
	}

	msg := fmt.Sprintf("receiver not found: %s", receiverName)
	log.Error(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func serveJson(w http.ResponseWriter, req *http.Request, obj interface{}) {
	wait := req.URL.RawQuery == "wait"

	if wait {
		// wait for change config trigger
		log.Debug("Wait for configCond: %s", configCond)
		configCond.L.Lock()
		configCond.Wait()
		configCond.L.Unlock()
	}

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
	log.Info("serve: %s", req.URL.Path)

	if req.URL.Path == "/" {
		localPath := *staticDir + "/index.html"
		http.ServeFile(w, req, localPath)
	} else if req.URL.Path == "/api/config" {
		serveJson(w, req, config)
	} else if req.URL.Path == "/api/backends" {
		serveJson(w, req, backends)
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

func loadBackendsConfig(backendFile *string) {
	log.Info("loading backend file %s", *backendFile)
	d, err := ioutil.ReadFile(*backendFile)
	if err != nil {
		log.Error("error reading backends: %v", err)
		return
	}
	yaml.Unmarshal(d, &backends)
	log.Debug("backends: %s", backends)
}

func loadConfigCache(configFile *string) {
	if _, err := os.Stat(*configFile); err == nil {
		log.Info("loading config cache file %s", *configFile)
		d, err := ioutil.ReadFile(*configFile)
		if err != nil {
			log.Error("error reading config: %v", err)
			return
		}
		yaml.Unmarshal(d, &config)
		log.Debug("config: %s", config)
	} else {
		log.Info("create initial config")
		config.Servers = make(map[string]*Radio)
		config.Receivers = make(map[string]*Server)
		for _, s := range backends.Servers {
			config.Servers[s.Host] = backends.Radios[0]
		}
		for _, r := range backends.Receivers {
			config.Receivers[*r] = nil
		}
	}
}

func saveConfigCache(configFile *string) {
	d, err := yaml.Marshal(&config)
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
		configCond.L.Lock()
		configCond.Wait()
		configCond.L.Unlock()
		saveConfigCache(configFile)
	}
}

// MAIN --------------------------------------------

func main() {
	backendFile := flag.String("backends", "", "Backend config file")
	configFile := flag.String("config-cache", configCacheFile, "File for persisting config state")
	port := flag.Int("http", 8080, "Port for binding the config server")
	staticDir = flag.String("webroot", "static", "Directory for serving static content")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()
	initLogger(*verbose)


	if *backendFile == "" {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	log.Info("starting")
	locker := &sync.Mutex{}
	configCond = sync.NewCond(locker)

	loadBackendsConfig(backendFile)
	loadConfigCache(configFile)
	go scheduleSaveConfigCache(configFile)

	httpd(*port)

	saveConfigCache(configFile)
}
