package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	gmp "github.com/lqqyt2423/go-mitmproxy/proxy"
)

type ActionType string

const (
	ActionBlock         ActionType = "block"
	ActionMock          ActionType = "mock"
	ActionModifyHeaders ActionType = "modify_headers"
	ActionReplaceBody   ActionType = "replace_body"
	ActionManualReq     ActionType = "manual_req"
	ActionManualResp    ActionType = "manual_resp"
	ActionManualBoth    ActionType = "manual_both"
)

type Condition struct {
	URLPattern  string `json:"url_pattern,omitempty"`
	Host        string `json:"host,omitempty"`
	Method      string `json:"method,omitempty"`
	HeaderName  string `json:"header_name,omitempty"`
	HeaderValue string `json:"header_value,omitempty"`
	StatusCode  int    `json:"status_code,omitempty"` // response-phase only; 0=any
}

type Action struct {
	Type        ActionType        `json:"type"`
	StatusCode  int               `json:"status_code,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
}

type Rule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Condition Condition `json:"condition"`
	Action    Action    `json:"action"`
}

type RuleEngine struct {
	mu    sync.RWMutex
	rules []Rule
	dir   string
}

func NewRuleEngine(dir string) *RuleEngine {
	e := &RuleEngine{dir: dir}
	_ = e.Load()
	return e
}

func (e *RuleEngine) Load() error {
	data, err := os.ReadFile(filepath.Join(e.dir, "rules.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}
	e.mu.Lock()
	e.rules = rules
	e.mu.Unlock()
	return nil
}

func (e *RuleEngine) save() error {
	if err := os.MkdirAll(e.dir, 0700); err != nil {
		return err
	}
	e.mu.RLock()
	data, err := json.MarshalIndent(e.rules, "", "  ")
	e.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.dir, "rules.json"), data, 0600)
}

func (e *RuleEngine) List() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

func (e *RuleEngine) Add(r Rule) Rule {
	r.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	e.mu.Lock()
	e.rules = append(e.rules, r)
	e.mu.Unlock()
	_ = e.save()
	return r
}

func (e *RuleEngine) Update(r Rule) bool {
	e.mu.Lock()
	found := false
	for i, ex := range e.rules {
		if ex.ID == r.ID {
			e.rules[i] = r
			found = true
			break
		}
	}
	e.mu.Unlock()
	if found {
		_ = e.save()
	}
	return found
}

func (e *RuleEngine) Delete(id string) bool {
	e.mu.Lock()
	found := false
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			found = true
			break
		}
	}
	e.mu.Unlock()
	if found {
		_ = e.save()
	}
	return found
}

// ApplyRequest evaluates request-phase rules (block, mock).
// Returns the action type that fired, or "" if none matched.
func (e *RuleEngine) ApplyRequest(f *gmp.Flow) ActionType {
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Action.Type {
		case ActionBlock, ActionMock, ActionManualReq, ActionManualBoth:
		default:
			continue
		}
		if !matchRequest(r.Condition, f) {
			continue
		}
		switch r.Action.Type {
		case ActionBlock:
			f.Response = &gmp.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       []byte("Blocked by rule: " + r.Name),
			}
		case ActionMock:
			sc := r.Action.StatusCode
			if sc == 0 {
				sc = http.StatusOK
			}
			ct := r.Action.ContentType
			if ct == "" {
				ct = "application/json"
			}
			h := make(http.Header)
			h.Set("Content-Type", ct)
			for k, v := range r.Action.Headers {
				h.Set(k, v)
			}
			f.Response = &gmp.Response{
				StatusCode: sc,
				Header:     h,
				Body:       []byte(r.Action.Body),
			}
		}
		return r.Action.Type
	}
	return ""
}

// ApplyResponse evaluates response-phase rules. Returns the action type that fired, or "".
func (e *RuleEngine) ApplyResponse(f *gmp.Flow) ActionType {
	if f.Response == nil {
		return ""
	}
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Action.Type {
		case ActionModifyHeaders, ActionReplaceBody, ActionManualResp, ActionManualBoth:
		default:
			continue
		}
		if !matchRequest(r.Condition, f) {
			continue
		}
		if r.Condition.StatusCode != 0 && f.Response.StatusCode != r.Condition.StatusCode {
			continue
		}
		if f.Response.Header == nil {
			f.Response.Header = make(http.Header)
		}
		switch r.Action.Type {
		case ActionModifyHeaders:
			for k, v := range r.Action.Headers {
				f.Response.Header.Set(k, v)
			}
		case ActionReplaceBody:
			f.Response.Body = []byte(r.Action.Body)
			if r.Action.ContentType != "" {
				f.Response.Header.Set("Content-Type", r.Action.ContentType)
			}
		case ActionManualResp, ActionManualBoth:
			return r.Action.Type
		}
	}
	return ""
}

func matchRequest(c Condition, f *gmp.Flow) bool {
	if c.Method != "" && c.Method != f.Request.Method {
		return false
	}
	if c.Host != "" {
		host := f.Request.URL.Host
		if host == "" {
			host = f.Request.Raw().Host
		}
		if c.Host != host {
			return false
		}
	}
	if c.URLPattern != "" {
		re, err := regexp.Compile(c.URLPattern)
		if err != nil || !re.MatchString(f.Request.URL.String()) {
			return false
		}
	}
	if c.HeaderName != "" {
		val := f.Request.Header.Get(c.HeaderName)
		if val == "" {
			return false
		}
		if c.HeaderValue != "" && val != c.HeaderValue {
			return false
		}
	}
	return true
}
