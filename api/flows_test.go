package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"proxito/proxy"
)

func newFlowServer(ids ...string) *Server {
	store := proxy.NewStore(10)
	for _, id := range ids {
		store.Add(&proxy.Flow{ID: id})
	}
	return &Server{store: store, intercept: proxy.NewInterceptHub(), hub: NewHub()}
}

// waitPaused simulates the proxy goroutine blocked on a paused flow and
// returns a channel that receives the resume decision.
func waitPaused(s *Server, id string) <-chan proxy.ResumeMsg {
	done := make(chan proxy.ResumeMsg, 1)
	go func() { done <- s.intercept.Wait(id) }()
	// Wait registers the pending channel asynchronously; give it a moment.
	time.Sleep(20 * time.Millisecond)
	return done
}

func expectResumed(t *testing.T, done <-chan proxy.ResumeMsg) {
	t.Helper()
	select {
	case msg := <-done:
		if !msg.OK {
			t.Error("paused flow was blocked, want resumed")
		}
	case <-time.After(time.Second):
		t.Fatal("paused flow was never released")
	}
}

func deleteFlow(s *Server, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/flows/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	s.handleDeleteFlow(rec, req)
	return rec
}

func TestDeleteFlow_shouldRemoveFlowFromStore(t *testing.T) {
	// Arrange
	s := newFlowServer("a", "b")
	// Act
	rec := deleteFlow(s, "a")
	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if _, found := s.store.Get("a"); found {
		t.Error("flow a still in store")
	}
	if _, found := s.store.Get("b"); !found {
		t.Error("flow b was removed too")
	}
}

func TestDeleteFlow_shouldReturn404WhenFlowDoesNotExist(t *testing.T) {
	// Arrange
	s := newFlowServer("a")
	// Act
	rec := deleteFlow(s, "missing")
	// Assert
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteFlow_shouldResumePausedFlow(t *testing.T) {
	// Arrange
	s := newFlowServer("a")
	done := waitPaused(s, "a")
	// Act
	deleteFlow(s, "a")
	// Assert
	expectResumed(t, done)
}

func TestClearFlows_shouldEmptyStoreAndResumePausedFlows(t *testing.T) {
	// Arrange
	s := newFlowServer("a", "b")
	done := waitPaused(s, "b")
	rec := httptest.NewRecorder()
	// Act
	s.handleClearFlows(rec, httptest.NewRequest(http.MethodDelete, "/api/flows", nil))
	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if n := len(s.store.List()); n != 0 {
		t.Errorf("store has %d flows after clear, want 0", n)
	}
	expectResumed(t, done)
}
