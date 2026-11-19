package proxy

import (
	"reflect"
	"testing"
)

func storeWith(ids ...string) *Store {
	s := NewStore(10)
	for _, id := range ids {
		s.Add(&Flow{ID: id})
	}
	return s
}

func listIDs(s *Store) []string {
	ids := []string{}
	for _, f := range s.List() {
		ids = append(ids, f.ID)
	}
	return ids
}

func TestStoreDelete_shouldRemoveOnlyTheGivenFlow(t *testing.T) {
	// Arrange
	s := storeWith("a", "b", "c")
	// Act
	ok := s.Delete("b")
	// Assert
	if !ok {
		t.Fatal("Delete returned false for an existing flow")
	}
	if got := listIDs(s); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Errorf("List() = %v, want [a c]", got)
	}
	if _, found := s.Get("b"); found {
		t.Error("deleted flow still reachable via Get")
	}
}

func TestStoreDelete_shouldReturnFalseWhenFlowDoesNotExist(t *testing.T) {
	// Arrange
	s := storeWith("a")
	// Act
	ok := s.Delete("missing")
	// Assert
	if ok {
		t.Error("Delete returned true for a missing flow")
	}
	if got := listIDs(s); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("List() = %v, want [a]", got)
	}
}

func TestStoreClear_shouldRemoveAllFlowsAndReturnTheirIDs(t *testing.T) {
	// Arrange
	s := storeWith("a", "b")
	// Act
	removed := s.Clear()
	// Assert
	if !reflect.DeepEqual(removed, []string{"a", "b"}) {
		t.Errorf("Clear() = %v, want [a b]", removed)
	}
	if got := listIDs(s); len(got) != 0 {
		t.Errorf("List() after Clear = %v, want empty", got)
	}
	if _, found := s.Get("a"); found {
		t.Error("cleared flow still reachable via Get")
	}
}

func TestStoreClear_shouldKeepStoreUsableForNewFlows(t *testing.T) {
	// Arrange
	s := storeWith("a")
	s.Clear()
	// Act
	s.Add(&Flow{ID: "b"})
	// Assert
	if _, found := s.Get("b"); !found {
		t.Error("flow added after Clear not found")
	}
	if got := listIDs(s); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("List() = %v, want [b]", got)
	}
}
