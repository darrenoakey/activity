package proc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// HideList manages a persistent set of hidden process names.
type HideList struct {
	mu     sync.Mutex
	hidden map[string]bool
	path   string
}

// NewHideList creates a HideList backed by the given JSON file path.
// Loads existing entries if the file exists.
func NewHideList(path string) (*HideList, error) {
	h := &HideList{
		hidden: make(map[string]bool),
		path:   path,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading hide list: %w", err)
	}

	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return nil, fmt.Errorf("parsing hide list: %w", err)
	}

	for _, name := range names {
		h.hidden[name] = true
	}

	return h, nil
}

// IsHidden returns true if the given process name is in the hide list.
func (h *HideList) IsHidden(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hidden[name]
}

// Toggle adds the name if not hidden, removes it if already hidden.
// Returns true if the name is now hidden (was added).
func (h *HideList) Toggle(name string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.hidden[name] {
		delete(h.hidden, name)
	} else {
		h.hidden[name] = true
	}

	if err := h.save(); err != nil {
		return h.hidden[name], fmt.Errorf("saving hide list: %w", err)
	}

	return h.hidden[name], nil
}

// Count returns the number of hidden process names.
func (h *HideList) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hidden)
}

// save writes the hide list to disk. Caller must hold h.mu.
func (h *HideList) save() error {
	names := make([]string, 0, len(h.hidden))
	for name := range h.hidden {
		names = append(names, name)
	}

	data, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}

	dir := filepath.Dir(h.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	return os.WriteFile(h.path, data, 0o644)
}
