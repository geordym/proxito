package config

import (
	"reflect"
	"testing"
)

func TestSaveLoad_shouldPersistAutoAttachFields(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	want := &Config{
		JavaBin:            "/usr/bin/java",
		AutoAttach:         true,
		AutoAttachPatterns: []string{"com.foo.Demo"},
	}
	// Act
	if err := want.Save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(dir)
	// Assert
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoad_shouldDefaultAutoAttachOffWhenMissing(t *testing.T) {
	// Arrange / Act
	got, err := Load(t.TempDir())
	// Assert
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AutoAttach || len(got.AutoAttachPatterns) != 0 {
		t.Errorf("expected auto-attach off with empty list, got %+v", got)
	}
}
