package tui

import (
	"strings"

	"github.com/mark3labs/iteratr/internal/tui/theme"
)

// renderModalTitle renders a title with block pattern decoration.
// Creates format: "Title ▀▄▀▄▀▄▀▄" with a gradient from primary to secondary.
// Uses the same block characters as the logo (▀ ▄) for visual consistency.
func renderModalTitle(title string, width int) string {
	t := theme.Current()

	// Calculate remaining width for pattern
	titleLen := len(title)
	remainingWidth := width - titleLen - 1 // -1 for space
	if remainingWidth <= 0 {
		return t.S().ModalTitle.UnsetAlign().Render(title)
	}

	// Build pattern with block characters
	pattern := strings.Repeat("▄▀", remainingWidth/2)
	if remainingWidth%2 == 1 {
		pattern += "▄"
	}

	// Style title and apply gradient to pattern
	titleStyle := t.S().ModalTitle.UnsetAlign()
	styledTitle := titleStyle.Render(title)
	styledPattern := theme.ApplyGradient(pattern, string(t.Primary), string(t.Secondary))

	return styledTitle + " " + styledPattern
}
