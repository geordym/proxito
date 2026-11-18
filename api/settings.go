package api

import (
	"encoding/json"
	"net/http"

	"proxito/config"
	"proxito/jvm"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"java_bin":          s.jvmMgr.GetJavaBin(),
		"proxy_host":        s.proxyHost,
		"proxy_port":        s.proxyPort,
		"java_auto_detected": s.javaAutoDetected,
		"java_auto_path":    s.javaAutoPath,
		"jdks":              s.jdks,
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var body config.Config
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	if body.JavaBin != "" {
		s.jvmMgr.SetJavaBin(body.JavaBin)
		s.javaAutoDetected = false // user chose manually
	}

	cfg := &config.Config{
		JavaBin:   s.jvmMgr.GetJavaBin(),
		ProxyHost: s.proxyHost,
		ProxyPort: s.proxyPort,
	}
	if err := cfg.Save(s.cfgDir); err != nil {
		jsonErr(w, "no pude guardar la configuración: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Broadcast(map[string]any{
		"type":     "settings_changed",
		"java_bin": cfg.JavaBin,
	})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) refreshJDKs() {
	s.jdks = jvm.DetectJDKs()
}
