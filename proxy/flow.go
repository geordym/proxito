package proxy

import (
	"net/http"
	"sync"
	"time"
)

type State string

const (
	StateActive  State = "active"
	StatePaused  State = "paused"
	StateDone    State = "done"
	StateBlocked State = "blocked"
	StateError   State = "error"
)

// Flow represents a single HTTP request/response pair.
type Flow struct {
	ID          string
	Method      string
	URL         string
	Host        string
	Path        string
	ReqHeaders  http.Header
	ReqBody     []byte
	Status      int
	RespHeaders http.Header
	RespBody    []byte
	ContentType string
	Size        int
	State       State
	StartedAt   time.Time
	DoneAt      time.Time
}

func (f *Flow) DurationMs() int64 {
	if f.DoneAt.IsZero() {
		return 0
	}
	return f.DoneAt.Sub(f.StartedAt).Milliseconds()
}

// Store is a thread-safe, size-bounded list of flows.
type Store struct {
	mu    sync.RWMutex
	flows []*Flow
	byID  map[string]*Flow
	maxN  int
}

func NewStore(maxN int) *Store {
	return &Store{byID: make(map[string]*Flow), maxN: maxN}
}

func (s *Store) Add(f *Flow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows = append(s.flows, f)
	s.byID[f.ID] = f
	if len(s.flows) > s.maxN {
		old := s.flows[0]
		s.flows = s.flows[1:]
		delete(s.byID, old.ID)
	}
}

func (s *Store) Get(id string) (*Flow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.byID[id]
	return f, ok
}

func (s *Store) List() []*Flow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Flow, len(s.flows))
	copy(out, s.flows)
	return out
}

// Delete removes the flow with the given id and reports whether it existed.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	delete(s.byID, id)
	for i, f := range s.flows {
		if f.ID == id {
			s.flows = append(s.flows[:i], s.flows[i+1:]...)
			break
		}
	}
	return true
}

// Clear removes every flow and returns the ids that were removed.
func (s *Store) Clear() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, len(s.flows))
	for i, f := range s.flows {
		ids[i] = f.ID
	}
	s.flows = nil
	s.byID = make(map[string]*Flow)
	return ids
}

func (s *Store) SetPaused(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.byID[id]; ok {
		f.State = StatePaused
	}
}

func (s *Store) SetResponse(id string, status int, headers http.Header, body []byte, ct string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.byID[id]
	if !ok {
		return
	}
	f.Status = status
	f.RespHeaders = headers
	f.RespBody = body
	f.ContentType = ct
	f.Size = len(body)
	f.State = StateDone
	f.DoneAt = time.Now()
}

func (s *Store) SetBlocked(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.byID[id]; ok {
		f.State = StateBlocked
		f.DoneAt = time.Now()
	}
}

func (s *Store) SetError(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.byID[id]; ok {
		f.State = StateError
		f.DoneAt = time.Now()
	}
}

// ResumeCmd carries the user's decision for a paused flow plus optional edits.
type ResumeCmd struct {
	Resume  bool
	Headers map[string]string
	Body    []byte
}

// InterceptHub manages pause/resume of individual flows.
type InterceptHub struct {
	mu      sync.Mutex
	pending map[string]chan ResumeCmd
	enabled bool
}

func NewInterceptHub() *InterceptHub {
	return &InterceptHub{pending: make(map[string]chan ResumeCmd)}
}

func (h *InterceptHub) Enabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enabled
}

func (h *InterceptHub) SetEnabled(v bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = v
}

// Wait blocks the calling goroutine until Resume or Block is called for id.
func (h *InterceptHub) Wait(id string) ResumeCmd {
	ch := make(chan ResumeCmd, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	return <-ch
}

func (h *InterceptHub) Resume(id string, headers map[string]string, body []byte) {
	h.send(id, ResumeCmd{Resume: true, Headers: headers, Body: body})
}

func (h *InterceptHub) Block(id string) {
	h.send(id, ResumeCmd{Resume: false})
}

func (h *InterceptHub) send(id string, cmd ResumeCmd) {
	h.mu.Lock()
	ch, ok := h.pending[id]
	if ok {
		delete(h.pending, id)
	}
	h.mu.Unlock()
	if ok {
		ch <- cmd
	}
}
