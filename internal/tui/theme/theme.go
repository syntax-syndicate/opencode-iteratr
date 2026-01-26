package theme

import (
	"sync"

	"charm.land/lipgloss/v2"
)

// Theme defines the color palette for the TUI.
// For tracer bullet, this contains only minimal colors needed to validate architecture.
type Theme struct {
	Name   string
	IsDark bool

	// Semantic colors
	Primary string // lipgloss.Color is a string type

	// Background hierarchy
	BgBase string

	// Foreground hierarchy
	FgBase string

	// Lazy-built styles
	styles     *Styles
	stylesOnce sync.Once
}

// S returns the pre-built styles for this theme.
// Styles are lazily initialized on first call.
func (t *Theme) S() *Styles {
	t.stylesOnce.Do(func() {
		t.styles = t.buildStyles()
	})
	return t.styles
}

// buildStyles constructs the pre-built styles from theme colors.
func (t *Theme) buildStyles() *Styles {
	return &Styles{
		HeaderTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Primary)).
			Bold(true),
	}
}
