package proxy

import (
	"net/http"
	"net/url"
	"time"

	gmp "github.com/lqqyt2423/go-mitmproxy/proxy"
)

// Addon wires go-mitmproxy flows into our Store and InterceptHub.
type Addon struct {
	gmp.BaseAddon
	store     *Store
	intercept *InterceptHub
	rules     *RuleEngine
	broadcast func(any)
}

func NewAddon(store *Store, intercept *InterceptHub, rules *RuleEngine, broadcast func(any)) *Addon {
	return &Addon{store: store, intercept: intercept, rules: rules, broadcast: broadcast}
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

	if a.rules != nil {
		switch a.rules.ApplyRequest(f) {
		case ActionBlock:
			a.store.SetBlocked(id)
			a.broadcast(map[string]any{"type": "flow_done", "id": id, "state": string(StateBlocked)})
			return
		case ActionMock:
			ct := f.Response.Header.Get("Content-Type")
			a.store.SetResponse(id, f.Response.StatusCode, f.Response.Header.Clone(), f.Response.Body, ct)
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
			return
		case ActionManualReq, ActionManualBoth:
			a.store.SetPaused(id)
			a.store.SetPausedAt(id, "request")
			a.broadcast(map[string]any{"type": "flow_paused", "id": id, "state": string(StatePaused), "paused_at": "request"})
			msg := a.intercept.Wait(id)
			if !msg.OK {
				a.store.SetBlocked(id)
				a.broadcast(map[string]any{"type": "flow_done", "id": id, "state": string(StateBlocked)})
				f.Response = &gmp.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: []byte("Blocked by Proxito")}
				return
			}
			if len(msg.Headers) > 0 {
				for k, v := range msg.Headers {
					f.Request.Header.Set(k, v)
				}
			}
			if len(msg.QueryParams) > 0 {
				q := url.Values{}
				for k, v := range msg.QueryParams {
					q.Set(k, v)
				}
				f.Request.URL.RawQuery = q.Encode()
			}
			if msg.Body != nil {
				f.Request.Body = msg.Body
			}
			a.store.UpdateRequest(id, msg.Headers, msg.Body)
			a.broadcast(map[string]any{"type": "flow_resumed", "id": id, "state": string(StateActive)})
		}
	}
}

func (a *Addon) Response(f *gmp.Flow) {
	if f.Response == nil {
		return
	}

	var respAction ActionType
	if a.rules != nil {
		respAction = a.rules.ApplyResponse(f)
	}

	id := f.Id.String()

	if respAction == ActionManualResp || respAction == ActionManualBoth {
		ct := f.Response.Header.Get("Content-Type")
		a.store.SetResponse(id, f.Response.StatusCode, f.Response.Header.Clone(), f.Response.Body, ct)
		a.store.SetPaused(id)
		a.store.SetPausedAt(id, "response")
		a.broadcast(map[string]any{"type": "flow_paused", "id": id, "state": string(StatePaused), "paused_at": "response"})
		msg := a.intercept.Wait(id)
		if !msg.OK {
			f.Response = &gmp.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: []byte("Blocked by Proxito")}
			a.store.SetBlocked(id)
			a.broadcast(map[string]any{"type": "flow_done", "id": id, "state": string(StateBlocked)})
			return
		}
		if msg.RespStatus > 0 {
			f.Response.StatusCode = msg.RespStatus
		}
		if len(msg.RespHeaders) > 0 {
			for k, v := range msg.RespHeaders {
				f.Response.Header.Set(k, v)
			}
		}
		if msg.RespBody != nil {
			f.Response.Body = msg.RespBody
		}
		a.store.UpdateResponse(id, msg.RespStatus, msg.RespHeaders, msg.RespBody)
		a.broadcast(map[string]any{"type": "flow_resumed", "id": id, "state": string(StateActive)})
	}

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
