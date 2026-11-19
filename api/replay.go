package api

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"proxito/proxy"
)

var replaySkip = map[string]bool{
	"proxy-connection":  true,
	"transfer-encoding": true,
	"content-length":    true,
	"host":              true,
}

var replayClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	},
	Timeout: 30 * time.Second,
}

func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	orig, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		jsonErr(w, "flow not found", http.StatusNotFound)
		return
	}

	var bodyReader io.Reader
	if len(orig.ReqBody) > 0 {
		bodyReader = bytes.NewReader(orig.ReqBody)
	}

	req, err := http.NewRequestWithContext(r.Context(), orig.Method, orig.URL, bodyReader)
	if err != nil {
		jsonErr(w, "invalid flow url", http.StatusInternalServerError)
		return
	}
	for k, vals := range orig.ReqHeaders {
		if replaySkip[strings.ToLower(k)] {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	newID := fmt.Sprintf("replay-%d", time.Now().UnixNano())
	now := time.Now()
	newFlow := &proxy.Flow{
		ID:         newID,
		Method:     orig.Method,
		URL:        orig.URL,
		Host:       orig.Host,
		Path:       orig.Path,
		ReqHeaders: orig.ReqHeaders,
		ReqBody:    orig.ReqBody,
		State:      proxy.StateActive,
		StartedAt:  now,
	}
	s.store.Add(newFlow)
	s.hub.Broadcast(map[string]any{
		"type":       "flow_request",
		"id":         newID,
		"method":     newFlow.Method,
		"url":        newFlow.URL,
		"host":       newFlow.Host,
		"path":       newFlow.Path,
		"state":      string(proxy.StateActive),
		"started_at": now,
	})

	resp, err := replayClient.Do(req)
	if err != nil {
		s.store.SetError(newID)
		s.hub.Broadcast(map[string]any{"type": "flow_done", "id": newID, "state": string(proxy.StateError)})
		jsonOK(w, map[string]string{"id": newID})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	doneAt := time.Now()
	s.store.SetResponse(newID, resp.StatusCode, resp.Header.Clone(), body, ct)
	s.hub.Broadcast(map[string]any{
		"type":         "flow_response",
		"id":           newID,
		"method":       newFlow.Method,
		"url":          newFlow.URL,
		"host":         newFlow.Host,
		"path":         newFlow.Path,
		"status":       resp.StatusCode,
		"content_type": ct,
		"size":         len(body),
		"started_at":   now,
		"done_at":      doneAt,
		"state":        string(proxy.StateDone),
	})

	jsonOK(w, map[string]string{"id": newID})
}
