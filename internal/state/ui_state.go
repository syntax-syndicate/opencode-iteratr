package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/iteratr/internal/logger"
)

// UIState holds persistent UI preferences that carry across sessions.
type UIState struct {
	Sidebar SidebarState `json:"sidebar"`
}

// SidebarState holds sidebar visibility preference.
type SidebarState struct {
	Visible bool `json:"visible"`
}

// DefaultUIState returns the default UI state with sensible defaults.
func DefaultUIState() *UIState {
	return &UIState{
		Sidebar: SidebarState{
			Visible: true, // Sidebar visible by default in desktop mode
		},
	}
}

// Load reads the UI state from .iteratr/ui-state.json.
// Returns default state if the file doesn't exist or on error.
func Load(dataDir string) *UIState {
	path := filepath.Join(dataDir, "ui-state.json")

	// If file doesn't exist, return defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultUIState()
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Warn("Failed to read UI state file: %v", err)
		return DefaultUIState()
	}

	// Parse JSON
	var state UIState
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Warn("Failed to parse UI state JSON: %v", err)
		return DefaultUIState()
	}

	return &state
}

// Save writes the UI state to .iteratr/ui-state.json.
// Creates the data directory if it doesn't exist.
func Save(dataDir string, state *UIState) error {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	path := filepath.Join(dataDir, "ui-state.json")

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling UI state: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing UI state file: %w", err)
	}

	logger.Debug("UI state saved to %s", path)
	return nil
}
