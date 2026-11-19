package jvm

import (
	"context"
	"log"
	"strings"
	"time"
)

// ProcessName extracts the process identifier from a list-targets description:
// the first token, i.e. the main class (com.foo.Demo) or the launched jar path.
func ProcessName(desc string) string {
	fields := strings.Fields(desc)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// watchList holds the auto-attach state. Callers must hold Manager.mu.
type watchList struct {
	enabled  bool
	patterns []string
}

func (w *watchList) contains(name string) bool {
	if name == "" {
		return false
	}
	for _, p := range w.patterns {
		if p == name {
			return true
		}
	}
	return false
}

// normalizePatterns trims, drops empties and removes duplicates keeping order.
func normalizePatterns(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// SetAutoAttach replaces the auto-attach flag and the watch list.
func (m *Manager) SetAutoAttach(enabled bool, patterns []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watch.enabled = enabled
	m.watch.patterns = normalizePatterns(patterns)
}

// AutoAttach returns the auto-attach flag and a copy of the watch list.
func (m *Manager) AutoAttach() (bool, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watch.enabled, append([]string{}, m.watch.patterns...)
}

// WatchIfEnabled adds name to the watch list when auto-attach is enabled.
// It reports whether the list changed.
func (m *Manager) WatchIfEnabled(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.watch.enabled || name == "" || m.watch.contains(name) {
		return false
	}
	m.watch.patterns = append(m.watch.patterns, name)
	return true
}

// FindByPID returns the running JVM with the given pid, if any.
func (m *Manager) FindByPID(pid string) (JVM, bool) {
	jvms, err := m.List()
	if err != nil {
		return JVM{}, false
	}
	for _, j := range jvms {
		if j.PID == pid {
			return j, true
		}
	}
	return JVM{}, false
}

// AttachTarget is where the proxy agent must point the JVM.
type AttachTarget struct {
	Host, Port, CertPath string
}

// autoAttachCandidates returns watched, not-yet-attached JVMs that have not
// already failed an auto-attach attempt.
func autoAttachCandidates(jvms []JVM, failed map[string]bool) []JVM {
	var out []JVM
	for _, j := range jvms {
		if j.Watched && !j.Attached && !failed[j.PID] {
			out = append(out, j)
		}
	}
	return out
}

// RunAutoAttach polls running JVMs every interval and attaches every watched
// process not yet attached. onAttached is called after each success. It blocks
// until ctx is cancelled.
func (m *Manager) RunAutoAttach(ctx context.Context, interval time.Duration, target AttachTarget, onAttached func(JVM)) {
	failed := make(map[string]bool)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.autoAttachTick(target, failed, onAttached)
		}
	}
}

func (m *Manager) autoAttachTick(target AttachTarget, failed map[string]bool, onAttached func(JVM)) {
	if enabled, patterns := m.AutoAttach(); !enabled || len(patterns) == 0 {
		return
	}
	jvms, err := m.List()
	if err != nil {
		log.Printf("auto-attach: no pude listar JVMs: %v", err)
		return
	}
	for _, j := range autoAttachCandidates(jvms, failed) {
		if err := m.Attach(j.PID, target.Host, target.Port, target.CertPath); err != nil {
			if err != ErrAttachInProgress {
				failed[j.PID] = true
				log.Printf("auto-attach: falló PID %s (%s): %v", j.PID, j.Name, err)
			}
			continue
		}
		j.Attached = true
		onAttached(j)
	}
}
