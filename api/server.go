package api

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"

	"proxito/jvm"
	"proxito/proxy"
)

// Server serves the web UI and REST/WebSocket API.
type Server struct {
	store            *proxy.Store
	intercept        *proxy.InterceptHub
	jvmMgr           *jvm.Manager
	hub              *Hub
	certPath         string
	proxyHost        string
	proxyPort        string
	cfgDir           string
	cfgMu            sync.Mutex
	jdks             []jvm.JDKInfo
	javaAutoDetected bool   // true if java was picked automatically on this run
	javaAutoPath     string // the path that was auto-selected
	frontendFS       embed.FS
}

func NewServer(
	store *proxy.Store,
	intercept *proxy.InterceptHub,
	jvmMgr *jvm.Manager,
	certPath, proxyHost, proxyPort, cfgDir string,
	javaAutoDetected bool,
	javaAutoPath string,
	frontendFS embed.FS,
) *Server {
	s := &Server{
		store:            store,
		intercept:        intercept,
		jvmMgr:           jvmMgr,
		hub:              NewHub(),
		certPath:         certPath,
		proxyHost:        proxyHost,
		proxyPort:        proxyPort,
		cfgDir:           cfgDir,
		javaAutoDetected: javaAutoDetected,
		javaAutoPath:     javaAutoPath,
		frontendFS:       frontendFS,
	}
	s.refreshJDKs()
	return s
}

func (s *Server) Hub() *Hub { return s.hub }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Frontend
	sub, _ := fs.Sub(s.frontendFS, "frontend")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	// JVM API
	mux.HandleFunc("GET /api/jvms", s.handleListJVMs)
	mux.HandleFunc("POST /api/jvms/{pid}/attach", s.handleAttach)

	// Flow API
	mux.HandleFunc("GET /api/flows", s.handleListFlows)
	mux.HandleFunc("GET /api/flows/{id}", s.handleGetFlow)
	mux.HandleFunc("POST /api/flows/{id}/resume", s.handleResume)
	mux.HandleFunc("POST /api/flows/{id}/block", s.handleBlock)

	// Intercept toggle + cert download
	mux.HandleFunc("POST /api/intercept", s.handleIntercept)
	mux.HandleFunc("GET /api/cert", s.handleCert)

	// Settings
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleSaveSettings)

	// WebSocket
	mux.HandleFunc("GET /ws", s.hub.ServeWS)

	return mux
}
