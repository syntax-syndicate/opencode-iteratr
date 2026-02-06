package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/iteratr/internal/tui/theme"
)

// ToastDismissMsg is sent when the toast should be dismissed.
type ToastDismissMsg struct{}

// ShowToastMsg is sent to show a toast notification.
type ShowToastMsg struct {
	Text string
}

// Toast is a minimal toast notification component.
// Shows a message in the bottom-right corner that auto-dismisses after 3 seconds.
type Toast struct {
	message   string
	visible   bool
	dismissAt time.Time
}

// NewToast creates a new Toast component.
func NewToast() *Toast {
	return &Toast{}
}

// Show displays a toast with the given message.
// The toast will auto-dismiss after 3 seconds.
func (t *Toast) Show(msg string) tea.Cmd {
	t.message = msg
	t.visible = true
	t.dismissAt = time.Now().Add(3 * time.Second)
	return t.dismissCmd()
}

// dismissCmd returns a command that will dismiss the toast after the remaining time.
func (t *Toast) dismissCmd() tea.Cmd {
	remaining := time.Until(t.dismissAt)
	if remaining <= 0 {
		remaining = 1 * time.Millisecond
	}
	return tea.Tick(remaining, func(time.Time) tea.Msg {
		return ToastDismissMsg{}
	})
}

// Update handles messages for the toast component.
// Returns a command to re-schedule dismissal if needed.
func (t *Toast) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case ToastDismissMsg:
		t.visible = false
		t.message = ""
		return nil
	}
	return nil
}

// View renders the toast at the given screen dimensions.
// Returns empty string if toast is not visible.
// Positions the toast in the bottom-right corner, above the status bar.
func (t *Toast) View(width, height int) string {
	if !t.visible || t.message == "" {
		return ""
	}

	th := theme.Current()

	// Style the toast with warning colors (yellow) to indicate notification
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.FgBase)).
		Background(lipgloss.Color(th.Warning)).
		Padding(0, 1).
		Bold(true)

	content := style.Render(t.message)

	// Calculate position: bottom-right with 1 cell padding from edges
	// The toast appears above the status bar, so we position at height-2
	contentWidth := lipgloss.Width(content)
	if contentWidth > width-2 {
		content = style.Width(width - 2).Render(t.message)
	}

	// Position at row height-2 (1 row above status bar)
	verticalPadding := height - 2
	if verticalPadding < 0 {
		verticalPadding = 0
	}

	// Build the positioned toast with bottom-right alignment
	var result string
	for i := 0; i < verticalPadding; i++ {
		result += "\n"
	}
	result += lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Right).
		PaddingRight(1).
		Render(content)

	return result
}

// IsVisible returns whether the toast is currently visible.
func (t *Toast) IsVisible() bool {
	return t.visible
}

// GetMessage returns the current toast message (empty if not visible).
func (t *Toast) GetMessage() string {
	if !t.visible {
		return ""
	}
	return t.message
}
