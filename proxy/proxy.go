package proxy

import (
	"net/http"
	"time"

	gmp "github.com/lqqyt2423/go-mitmproxy/proxy"
)

// Addon wires go-mitmproxy flows into our Store and InterceptHub.
type Addon struct {
	gmp.BaseAddon
	store     *Store
	intercept *InterceptHub
	broadcast func(any)
}

func NewAddon(store *Store, intercept *InterceptHub, broadcast func(any)) *Addon {
	return &Addon{store: store, intercept: intercept, broadcast: broadcast}
}

func (a *Addon) Request(f *gmp.Flow) {
	id := f.Id.String()

	host := f.Request.URL.Host
	if host == "" {
		host = f.Request.Raw().Host
	}

	pf := &Flow{
		ID:         id,
		Method:     f.Request.Method,
		URL:        f.Request.URL.String(),
		Host:       host,
		Path:       f.Request.URL.Path,
		ReqHeaders: f.Request.Header.Clone(),
		ReqBody:    f.Request.Body,
		State:      StateActive,
		StartedAt:  f.StartTime,
	}
	a.store.Add(pf)

	a.broadcast(map[string]any{
		"type":       "flow_request",
		"id":         id,
		"method":     pf.Method,
		"url":        pf.URL,
		"host":       pf.Host,
		"path":       pf.Path,
		"state":      string(StateActive),
		"started_at": pf.StartedAt,
		"size":       0,
		"status":     0,
	})

	if a.intercept.Enabled() {
		a.store.SetPaused(id)
		a.broadcast(map[string]any{"type": "flow_paused", "id": id, "state": string(StatePaused)})

		resume := a.intercept.Wait(id)
		if !resume {
			a.store.SetBlocked(id)
			a.broadcast(map[string]any{"type": "flow_done", "id": id, "state": string(StateBlocked)})
			f.Response = &gmp.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       []byte("Blocked by Proxito"),
			}
			return
		}
		a.broadcast(map[string]any{"type": "flow_resumed", "id": id, "state": string(StateActive)})
	}
}

func (a *Addon) Response(f *gmp.Flow) {
	if f.Response == nil {
		return
	}
	id := f.Id.String()

	ct := f.Response.Header.Get("Content-Type")
	a.store.SetResponse(id, f.Response.StatusCode, f.Response.Header.Clone(), f.Response.Body, ct)

	pf, ok := a.store.Get(id)
	if !ok {
		return
	}

	a.broadcast(map[string]any{
		"type":         "flow_response",
		"id":           id,
		"method":       pf.Method,
		"url":          pf.URL,
		"host":         pf.Host,
		"path":         pf.Path,
		"status":       f.Response.StatusCode,
		"content_type": ct,
		"size":         len(f.Response.Body),
		"started_at":   pf.StartedAt,
		"done_at":      time.Now(),
		"state":        string(StateDone),
	})
}
