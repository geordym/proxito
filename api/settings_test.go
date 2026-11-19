package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"proxito/config"
	"proxito/jvm"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		jvmMgr: jvm.New("agent.jar", "/usr/bin/java"),
		hub:    NewHub(),
		cfgDir: t.TempDir(),
	}
}

func postSettings(s *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleSaveSettings(rec, req)
	return rec
}

func TestSaveSettings_shouldUpdateOnlyAutoAttachKeepingJavaBin(t *testing.T) {
	// Arrange
	s := newTestServer(t)
	// Act
	rec := postSettings(s, `{"auto_attach":true}`)
	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := s.jvmMgr.GetJavaBin(); got != "/usr/bin/java" {
		t.Errorf("java_bin changed to %q", got)
	}
	if enabled, _ := s.jvmMgr.AutoAttach(); !enabled {
		t.Error("expected auto-attach enabled")
	}
}

func TestSaveSettings_shouldReplaceWatchListAndPersist(t *testing.T) {
	// Arrange
	s := newTestServer(t)
	s.jvmMgr.SetAutoAttach(true, []string{"com.a.App", "com.b.Other"})
	// Act
	rec := postSettings(s, `{"auto_attach_patterns":["com.b.Other"]}`)
	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	saved, err := config.Load(s.cfgDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !saved.AutoAttach || !reflect.DeepEqual(saved.AutoAttachPatterns, []string{"com.b.Other"}) {
		t.Errorf("persisted %+v", saved)
	}
}

func TestSaveSettings_shouldRejectInvalidBody(t *testing.T) {
	// Arrange
	s := newTestServer(t)
	// Act
	rec := postSettings(s, `{not json`)
	// Assert
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
