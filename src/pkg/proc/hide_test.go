package proc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHideListPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hidden.json")

	// Create and add entries
	h, err := NewHideList(path)
	if err != nil {
		t.Fatalf("NewHideList: %v", err)
	}

	nowHidden, err := h.Toggle("kernel_task")
	if err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	if !nowHidden {
		t.Error("expected kernel_task to be hidden after toggle")
	}

	if !h.IsHidden("kernel_task") {
		t.Error("IsHidden should return true for kernel_task")
	}
	if h.IsHidden("not_hidden") {
		t.Error("IsHidden should return false for not_hidden")
	}

	// Verify file was written
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hide list file not created: %v", err)
	}

	// Load from disk in a new instance
	h2, err := NewHideList(path)
	if err != nil {
		t.Fatalf("NewHideList reload: %v", err)
	}

	if !h2.IsHidden("kernel_task") {
		t.Error("reloaded hide list should contain kernel_task")
	}

	// Toggle off
	nowHidden, err = h2.Toggle("kernel_task")
	if err != nil {
		t.Fatalf("Toggle off: %v", err)
	}
	if nowHidden {
		t.Error("expected kernel_task to be unhidden after second toggle")
	}

	if h2.Count() != 0 {
		t.Errorf("expected 0 hidden, got %d", h2.Count())
	}
}

func TestHideListNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "hidden.json")

	h, err := NewHideList(path)
	if err != nil {
		t.Fatalf("NewHideList: %v", err)
	}

	if h.Count() != 0 {
		t.Errorf("new hide list should be empty, got %d", h.Count())
	}

	_, err = h.Toggle("test_process")
	if err != nil {
		t.Fatalf("Toggle: %v", err)
	}

	// Verify subdirectory was created
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("subdirectory not created: %v", err)
	}
}
