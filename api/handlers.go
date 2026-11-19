package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"proxito/jvm"
)

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleListJVMs(w http.ResponseWriter, r *http.Request) {
	jvms, err := s.jvmMgr.List()
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if jvms == nil {
		jvms = []jvm.JVM{}
	}
	jsonOK(w, jvms)
}

func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")
	if err := s.jvmMgr.Attach(pid, s.proxyHost, s.proxyPort, s.certPath); err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hub.Broadcast(map[string]any{"type": "jvm_attached", "pid": pid})
	s.watchAttached(pid)
	jsonOK(w, map[string]string{"status": "ok"})
}

// watchAttached adds a manually attached JVM to the auto-attach watch list
// when auto-attach is enabled, and persists the change.
func (s *Server) watchAttached(pid string) {
	if enabled, _ := s.jvmMgr.AutoAttach(); !enabled {
		return
	}
	j, ok := s.jvmMgr.FindByPID(pid)
	if !ok || !s.jvmMgr.WatchIfEnabled(j.Name) {
		return
	}
	if err := s.saveConfig(); err != nil {
		log.Printf("auto-attach: no pude persistir el watch list: %v", err)
	}
	s.broadcastSettings()
}

func (s *Server) handleListFlows(w http.ResponseWriter, r *http.Request) {
	flows := s.store.List()
	out := make([]map[string]any, 0, len(flows))
	for _, f := range flows {
		out = append(out, map[string]any{
			"id":           f.ID,
			"method":       f.Method,
			"host":         f.Host,
			"path":         f.Path,
			"url":          f.URL,
			"status":       f.Status,
			"state":        string(f.State),
			"content_type": f.ContentType,
			"size":         f.Size,
			"duration_ms":  f.DurationMs(),
			"ts":           f.StartedAt.UnixMilli(),
		})
	}
	jsonOK(w, out)
}

func (s *Server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, ok := s.store.Get(id)
	if !ok {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]any{
		"id":           f.ID,
		"method":       f.Method,
		"url":          f.URL,
		"req_headers":  flatHeaders(f.ReqHeaders),
		"req_body":     string(f.ReqBody),
		"status":       f.Status,
		"resp_headers": flatHeaders(f.RespHeaders),
		"resp_body":    string(f.RespBody),
		"content_type": f.ContentType,
		"size":         f.Size,
		"state":        string(f.State),
		"duration_ms":  f.DurationMs(),
	})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.intercept.Resume(r.PathValue("id"))
	jsonOK(w, map[string]string{"status": "ok"})
}

// handleDeleteFlow removes one flow. A paused flow is resumed first so the
// client connection waiting on it is not left hanging.
func (s *Server) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.store.Delete(id) {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	s.intercept.Resume(id)
	s.hub.Broadcast(map[string]any{"type": "flow_deleted", "id": id})
	jsonOK(w, map[string]string{"status": "ok"})
}

// handleClearFlows removes every flow, resuming any that were paused.
func (s *Server) handleClearFlows(w http.ResponseWriter, r *http.Request) {
	for _, id := range s.store.Clear() {
		s.intercept.Resume(id)
	}
	s.hub.Broadcast(map[string]any{"type": "flows_cleared"})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	s.intercept.Block(r.PathValue("id"))
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleIntercept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.intercept.SetEnabled(body.Enabled)
	s.hub.Broadcast(map[string]any{"type": "intercept_changed", "enabled": body.Enabled})
	jsonOK(w, map[string]bool{"enabled": body.Enabled})
}

func (s *Server) handleCert(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.certPath)
	if err != nil {
		jsonErr(w, "cert not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="proxito-ca.pem"`)
	w.Write(data)
}

func flatHeaders(h map[string][]string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
