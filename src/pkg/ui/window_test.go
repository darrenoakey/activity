package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "window.json")

	wp, err := NewWindowPersist(path)
	if err != nil {
		t.Fatalf("NewWindowPersist: %v", err)
	}

	wp.UpdateSize(800, 500)

	// Wait for debounce to fire
	time.Sleep(700 * time.Millisecond)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("window state file not created: %v", err)
	}

	// Reload from disk
	wp2, err := NewWindowPersist(path)
	if err != nil {
		t.Fatalf("NewWindowPersist reload: %v", err)
	}

	wp2.mu.Lock()
	s := wp2.state
	wp2.mu.Unlock()

	if s.Width != 800 || s.Height != 500 {
		t.Errorf("reloaded state = %+v, want {800 500}", s)
	}
}

func TestWindowPersistNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "window.json")

	wp, err := NewWindowPersist(path)
	if err != nil {
		t.Fatalf("NewWindowPersist: %v", err)
	}

	wp.mu.Lock()
	s := wp.state
	wp.mu.Unlock()

	if s.Width != 900 || s.Height != 600 {
		t.Errorf("default state = %+v, want width=900 height=600", s)
	}
}
