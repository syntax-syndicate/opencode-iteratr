package tui

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
)

// TestStatusBar_AdaptToLayoutMode verifies StatusBar adapts to compact mode.
func TestStatusBar_AdaptToLayoutMode(t *testing.T) {
	tests := []struct {
		name       string
		mode       LayoutMode
		width      int
		expectFull bool // Expect full "connected" text vs just "●"
	}{
		{
			name:       "desktop mode shows full text",
			mode:       LayoutDesktop,
			width:      120,
			expectFull: true,
		},
		{
			name:       "compact mode shows condensed text",
			mode:       LayoutCompact,
			width:      80,
			expectFull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create status bar
			s := NewStatusBar("test-session")
			s.SetLayoutMode(tt.mode)
			s.SetSize(tt.width, 1)
			s.SetConnectionStatus(true)

			// Create a simple state with an in_progress task
			state := &session.State{
				Tasks: map[string]*session.Task{
					"task1": {
						ID:      "task1",
						Status:  "in_progress",
						Content: "This is a very long task name that should be truncated in compact mode",
					},
				},
			}
			s.SetState(state)

			// Create screen buffer and draw
			canvas := uv.NewScreenBuffer(tt.width, 1)
			area := uv.Rect(0, 0, tt.width, 1)
			s.Draw(canvas, area)

			// Get rendered content
			content := canvas.Render()

			// Verify based on mode
			if tt.expectFull {
				// Desktop mode should show "connected"
				if !strings.Contains(content, "connected") {
					t.Errorf("Desktop mode should contain 'connected' text, got: %q", content)
				}
			} else {
				// Compact mode should NOT show "connected" (just dot)
				if strings.Contains(content, "connected") {
					t.Errorf("Compact mode should not contain 'connected' text, got: %q", content)
				}
			}

			// Both modes should show the working indicator (spinner animation)
			// The spinner uses braille characters like ⠋, ⠙, ⠹, etc.
			hasSpinner := strings.ContainsAny(content, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
			if !hasSpinner {
				t.Errorf("Both modes should contain working indicator (spinner), got: %q", content)
			}
		})
	}
}

// TestFooter_AdaptToLayoutMode verifies Footer shows sidebar toggle in compact mode.
func TestFooter_AdaptToLayoutMode(t *testing.T) {
	tests := []struct {
		name             string
		mode             LayoutMode
		expectSidebarKey bool
	}{
		{
			name:             "desktop mode no sidebar toggle",
			mode:             LayoutDesktop,
			expectSidebarKey: false,
		},
		{
			name:             "compact mode shows sidebar toggle",
			mode:             LayoutCompact,
			expectSidebarKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create footer
			f := NewFooter()
			f.SetLayoutMode(tt.mode)
			f.SetSize(120, 1)

			// Create screen buffer and draw
			canvas := uv.NewScreenBuffer(120, 1)
			area := uv.Rect(0, 0, 120, 1)
			f.Draw(canvas, area)

			// Get rendered content
			content := canvas.Render()

			// Verify sidebar hint
			hasSidebarKey := strings.Contains(content, "[s]") && strings.Contains(content, "Sidebar")

			if tt.expectSidebarKey && !hasSidebarKey {
				t.Errorf("Compact mode should show '[s]Sidebar' hint, got: %q", content)
			}

			if !tt.expectSidebarKey && hasSidebarKey {
				t.Errorf("Desktop mode should not show '[s]Sidebar' hint, got: %q", content)
			}

			// Both modes should show view shortcuts
			if !strings.Contains(content, "[1]") || !strings.Contains(content, "[2]") {
				t.Errorf("Both modes should show view shortcuts, got: %q", content)
			}
		})
	}
}
