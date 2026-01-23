package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown content with syntax highlighting using glamour.
// Falls back to plain text wrapping if rendering fails.
func renderMarkdown(content string, width int) string {
	// Cap width to 120 for readability
	if width > 120 {
		width = 120
	}

	// Create glamour renderer with dark theme and word wrap
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to plain text wrapping
		return wrapText(content, width)
	}

	// Render the markdown
	rendered, err := r.Render(content)
	if err != nil {
		// Fallback to plain text wrapping
		return wrapText(content, width)
	}

	// Remove trailing newline that glamour adds
	return strings.TrimSuffix(rendered, "\n")
}
