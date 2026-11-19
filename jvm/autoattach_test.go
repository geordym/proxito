package jvm

import (
	"reflect"
	"testing"
)

func TestProcessName(t *testing.T) {
	cases := map[string]string{
		"com.foo.Demo --spring.profiles.active=dev": "com.foo.Demo",
		"/opt/app/service.jar":                      "/opt/app/service.jar",
		"   ":                                       "",
		"":                                          "",
	}
	for desc, want := range cases {
		// Arrange / Act
		got := ProcessName(desc)
		// Assert
		if got != want {
			t.Errorf("ProcessName(%q) = %q, want %q", desc, got, want)
		}
	}
}

func TestSetAutoAttach_shouldNormalizePatterns(t *testing.T) {
	// Arrange
	m := New("agent.jar", "java")
	// Act
	m.SetAutoAttach(true, []string{" com.a.App ", "", "com.a.App", "com.b.Other"})
	// Assert
	enabled, patterns := m.AutoAttach()
	if !enabled {
		t.Fatal("expected auto-attach enabled")
	}
	if want := []string{"com.a.App", "com.b.Other"}; !reflect.DeepEqual(patterns, want) {
		t.Errorf("patterns = %v, want %v", patterns, want)
	}
}

func TestAutoAttach_shouldReturnCopyOfPatterns(t *testing.T) {
	// Arrange
	m := New("agent.jar", "java")
	m.SetAutoAttach(true, []string{"com.a.App"})
	// Act
	_, patterns := m.AutoAttach()
	patterns[0] = "mutated"
	// Assert
	if _, again := m.AutoAttach(); again[0] != "com.a.App" {
		t.Errorf("internal watch list was mutated: %v", again)
	}
}

func TestWatchIfEnabled(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		existing    []string
		add         string
		wantChanged bool
		wantList    []string
	}{
		{"should add when enabled", true, nil, "com.a.App", true, []string{"com.a.App"}},
		{"should ignore when disabled", false, nil, "com.a.App", false, []string{}},
		{"should ignore duplicates", true, []string{"com.a.App"}, "com.a.App", false, []string{"com.a.App"}},
		{"should ignore empty name", true, nil, "", false, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			m := New("agent.jar", "java")
			m.SetAutoAttach(tt.enabled, tt.existing)
			// Act
			changed := m.WatchIfEnabled(tt.add)
			// Assert
			_, list := m.AutoAttach()
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !reflect.DeepEqual(list, tt.wantList) {
				t.Errorf("list = %v, want %v", list, tt.wantList)
			}
		})
	}
}

func TestAutoAttachCandidates_shouldSelectWatchedUnattachedNotFailed(t *testing.T) {
	// Arrange: two instances of the same watched app (must attach both),
	// one already attached, one that failed before, one not watched.
	jvms := []JVM{
		{PID: "1", Name: "com.a.App", Watched: true},
		{PID: "2", Name: "com.a.App", Watched: true},
		{PID: "3", Name: "com.a.App", Watched: true, Attached: true},
		{PID: "4", Name: "com.a.App", Watched: true},
		{PID: "5", Name: "com.b.Other"},
	}
	failed := map[string]bool{"4": true}
	// Act
	got := autoAttachCandidates(jvms, failed)
	// Assert
	var pids []string
	for _, j := range got {
		pids = append(pids, j.PID)
	}
	if want := []string{"1", "2"}; !reflect.DeepEqual(pids, want) {
		t.Errorf("candidates = %v, want %v", pids, want)
	}
}

func TestAttach_shouldBeNoOpWhenAlreadyAttached(t *testing.T) {
	// Arrange: a java binary that cannot run would fail if it were invoked.
	m := New("agent.jar", "/nonexistent/java")
	m.toolsDone = true
	m.attached["42"] = true
	// Act
	err := m.Attach("42", "127.0.0.1", "8080", "cert.pem")
	// Assert
	if err != nil {
		t.Errorf("expected no-op, got %v", err)
	}
}

func TestAttach_shouldRejectConcurrentAttachOnSamePID(t *testing.T) {
	// Arrange
	m := New("agent.jar", "/nonexistent/java")
	m.toolsDone = true
	m.inflight["42"] = true
	// Act
	err := m.Attach("42", "127.0.0.1", "8080", "cert.pem")
	// Assert
	if err != ErrAttachInProgress {
		t.Errorf("err = %v, want ErrAttachInProgress", err)
	}
}
