package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// SubagentModal displays a full-screen modal that loads and replays a subagent session.
// It reuses the existing ScrollList and MessageItem infrastructure from AgentOutput.
type SubagentModal struct {
	// Content display (reuses AgentOutput infrastructure)
	messages  []MessageItem
	toolIndex map[string]int // toolCallId → message index

	// Session metadata
	sessionID    string
	subagentType string
	workDir      string

	// ACP subprocess (populated by Start())
	cmd  interface{} // *exec.Cmd - will be set in Start()
	conn interface{} // *acpConn - will be set in Start()

	// State
	loading bool

	// Spinner for loading state (created lazily when needed)
	spinner *GradientSpinner

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSubagentModal creates a new SubagentModal.
func NewSubagentModal(sessionID, subagentType, workDir string) *SubagentModal {
	ctx, cancel := context.WithCancel(context.Background())
	spinner := NewDefaultGradientSpinner("Loading session...")
	return &SubagentModal{
		sessionID:    sessionID,
		subagentType: subagentType,
		workDir:      workDir,
		messages:     make([]MessageItem, 0),
		toolIndex:    make(map[string]int),
		loading:      true,
		ctx:          ctx,
		cancel:       cancel,
		spinner:      &spinner,
	}
}

// Start spawns the ACP subprocess, initializes it, and begins loading the session.
// Returns a command that will start the session loading process.
func (m *SubagentModal) Start() tea.Cmd {
	// This will be implemented in task TAS-16
	return nil
}

// Draw renders the modal as a full-screen overlay.
func (m *SubagentModal) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// This will be implemented in task TAS-19
	return nil
}

// Update handles keyboard input for scrolling.
func (m *SubagentModal) Update(msg tea.Msg) tea.Cmd {
	// This will be implemented in task TAS-20
	return nil
}

// HandleUpdate processes streaming messages from the subagent session.
// Returns a command to continue streaming if Continue is true.
func (m *SubagentModal) HandleUpdate(msg tea.Msg) tea.Cmd {
	// This will be implemented in task TAS-17 (continuous streaming)
	return nil
}

// Close terminates the ACP subprocess and cleans up resources.
// Safe to call multiple times or if Start() was never called.
func (m *SubagentModal) Close() {
	// Cancel context to stop any ongoing operations
	if m.cancel != nil {
		m.cancel()
	}

	// Close ACP connection if established
	// conn will be *acpConn when Start() is implemented
	if m.conn != nil {
		// Type assertion and close will be added in TAS-16
		// For now, this is a placeholder for the interface
		m.conn = nil
	}

	// Kill subprocess if running
	// cmd will be *exec.Cmd when Start() is implemented
	if m.cmd != nil {
		// Type assertion and process kill will be added in TAS-16
		// Pattern: cmd.Process.Kill() then cmd.Wait()
		m.cmd = nil
	}
}
