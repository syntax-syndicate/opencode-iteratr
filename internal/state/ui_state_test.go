package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultUIState(t *testing.T) {
	state := DefaultUIState()

	if state == nil {
		t.Fatal("DefaultUIState returned nil")
	}

	if !state.Sidebar.Visible {
		t.Error("Expected sidebar to be visible by default")
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Load from non-existent directory
	state := Load("/tmp/nonexistent-test-dir-xyz123")

	if state == nil {
		t.Fatal("Load returned nil for non-existent file")
	}

	// Should return defaults
	if !state.Sidebar.Visible {
		t.Error("Expected default sidebar visibility to be true")
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create and save state
	state := &UIState{
		Sidebar: SidebarState{
			Visible: false,
		},
	}

	err := Save(tmpDir, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "ui-state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load state back
	loaded := Load(tmpDir)

	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.Sidebar.Visible != false {
		t.Error("Loaded state does not match saved state")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Use subdirectory that doesn't exist
	dataDir := filepath.Join(tmpDir, "subdir", "data")

	state := DefaultUIState()
	err := Save(dataDir, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Error("Data directory was not created")
	}

	// Verify file exists
	path := filepath.Join(dataDir, "ui-state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("State file was not created")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Write invalid JSON
	path := filepath.Join(tmpDir, "ui-state.json")
	err := os.WriteFile(path, []byte("invalid json {{{"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Load should return defaults without crashing
	state := Load(tmpDir)

	if state == nil {
		t.Fatal("Load returned nil for invalid JSON")
	}

	// Should return defaults
	if !state.Sidebar.Visible {
		t.Error("Expected default sidebar visibility to be true when JSON is invalid")
	}
}
