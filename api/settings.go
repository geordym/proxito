package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"proxito/config"
	"proxito/jvm"
)

const autoAttachInterval = 5 * time.Second

// settingsUpdate is a partial update: nil fields are left untouched.
type settingsUpdate struct {
	JavaBin            *string   `json:"java_bin"`
	AutoAttach         *bool     `json:"auto_attach"`
	AutoAttachPatterns *[]string `json:"auto_attach_patterns"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	autoAttach, patterns := s.jvmMgr.AutoAttach()
	jsonOK(w, map[string]any{
		"java_bin":             s.jvmMgr.GetJavaBin(),
		"proxy_host":           s.proxyHost,
		"proxy_port":           s.proxyPort,
		"java_auto_detected":   s.javaAutoDetected,
		"java_auto_path":       s.javaAutoPath,
		"jdks":                 s.jdks,
		"auto_attach":          autoAttach,
		"auto_attach_patterns": patterns,
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	if body.JavaBin != nil && *body.JavaBin != "" {
		s.jvmMgr.SetJavaBin(*body.JavaBin)
		s.javaAutoDetected = false // user chose manually
	}
	if body.AutoAttach != nil || body.AutoAttachPatterns != nil {
		enabled, patterns := s.jvmMgr.AutoAttach()
		if body.AutoAttach != nil {
			enabled = *body.AutoAttach
		}
		if body.AutoAttachPatterns != nil {
			patterns = *body.AutoAttachPatterns
		}
		s.jvmMgr.SetAutoAttach(enabled, patterns)
	}

	if err := s.saveConfig(); err != nil {
		jsonErr(w, "no pude guardar la configuración: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.broadcastSettings()
	jsonOK(w, map[string]string{"status": "ok"})
}

// saveConfig persists the current runtime settings. Serialized so concurrent
// writers (settings form, manual attach) don't interleave.
func (s *Server) saveConfig() error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	autoAttach, patterns := s.jvmMgr.AutoAttach()
	cfg := &config.Config{
		JavaBin:            s.jvmMgr.GetJavaBin(),
		ProxyHost:          s.proxyHost,
		ProxyPort:          s.proxyPort,
		AutoAttach:         autoAttach,
		AutoAttachPatterns: patterns,
	}
	return cfg.Save(s.cfgDir)
}

func (s *Server) broadcastSettings() {
	autoAttach, patterns := s.jvmMgr.AutoAttach()
	s.hub.Broadcast(map[string]any{
		"type":                 "settings_changed",
		"java_bin":             s.jvmMgr.GetJavaBin(),
		"auto_attach":          autoAttach,
		"auto_attach_patterns": patterns,
	})
}

// StartAutoAttach runs the auto-attach watcher until ctx is cancelled and
// broadcasts jvm_auto_attached on every success.
func (s *Server) StartAutoAttach(ctx context.Context) {
	target := jvm.AttachTarget{Host: s.proxyHost, Port: s.proxyPort, CertPath: s.certPath}
	go s.jvmMgr.RunAutoAttach(ctx, autoAttachInterval, target, func(j jvm.JVM) {
		s.hub.Broadcast(map[string]any{"type": "jvm_auto_attached", "pid": j.PID, "name": j.Name})
	})
}

func (s *Server) refreshJDKs() {
	s.jdks = jvm.DetectJDKs()
}
