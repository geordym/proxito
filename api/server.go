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
	rules            *proxy.RuleEngine
	jvmMgr           *jvm.Manager
	hub              *Hub
	certPath         string
	proxyHost        string
	proxyPort        string
	cfgDir           string
	cfgMu            sync.Mutex
	jdks             []jvm.JDKInfo
	javaAutoDetected bool
	javaAutoPath     string
	frontendFS       embed.FS
}

func NewServer(
	store *proxy.Store,
	intercept *proxy.InterceptHub,
	rules *proxy.RuleEngine,
	jvmMgr *jvm.Manager,
	certPath, proxyHost, proxyPort, cfgDir string,
	javaAutoDetected bool,
	javaAutoPath string,
	frontendFS embed.FS,
) *Server {
	s := &Server{
		store:            store,
		intercept:        intercept,
		rules:            rules,
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
	mux.HandleFunc("DELETE /api/flows", s.handleClearFlows)
	mux.HandleFunc("GET /api/flows/{id}", s.handleGetFlow)
	mux.HandleFunc("DELETE /api/flows/{id}", s.handleDeleteFlow)
	mux.HandleFunc("POST /api/flows/{id}/resume", s.handleResume)
	mux.HandleFunc("POST /api/flows/{id}/block", s.handleBlock)

	// Intercept toggle + cert download
	mux.HandleFunc("POST /api/intercept", s.handleIntercept)
	mux.HandleFunc("GET /api/cert", s.handleCert)

	// Settings
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleSaveSettings)

	// Rules
	mux.HandleFunc("GET /api/rules", s.handleListRules)
	mux.HandleFunc("POST /api/rules", s.handleCreateRule)
	mux.HandleFunc("PUT /api/rules/{id}", s.handleUpdateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)

	// WebSocket
	mux.HandleFunc("GET /ws", s.hub.ServeWS)

	return mux
}
