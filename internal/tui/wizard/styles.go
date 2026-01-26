package wizard

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette (Catppuccin Mocha)
var (
	colorPrimary       = lipgloss.Color("#cba6f7") // Mauve
	colorSecondary     = lipgloss.Color("#b4befe") // Lavender
	colorText          = lipgloss.Color("#cdd6f4") // Text
	colorBase          = lipgloss.Color("#1e1e2e") // Base
	colorSubtext0      = lipgloss.Color("#a6adc8") // Subtext0
	colorSubtext1      = lipgloss.Color("#bac2de") // Subtext1
	colorSurface2      = lipgloss.Color("#585b70") // Surface2
	colorBorderFocused = lipgloss.Color("#b4befe") // Lavender for borders
)

// Modal styles (consistent with NoteInputModal)
var (
	styleModalContainer = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderFocused).
				Background(colorBase).
				Padding(1, 2)

	styleModalTitle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Align(lipgloss.Center)
)

// Hint bar styles
var (
	styleHintKey = lipgloss.NewStyle().
			Foreground(colorSubtext1).
			Bold(true)

	styleHintDesc = lipgloss.NewStyle().
			Foreground(colorSubtext0)

	styleHintSeparator = lipgloss.NewStyle().
				Foreground(colorSurface2)
)

// renderHintBar renders a hint bar with the given key-description pairs.
// Example: renderHintBar("↑↓", "navigate", "enter", "select", "esc", "back")
// Returns: "↑↓ navigate • enter select • esc back"
func renderHintBar(pairs ...string) string {
	if len(pairs) == 0 || len(pairs)%2 != 0 {
		return ""
	}

	var result string
	for i := 0; i < len(pairs); i += 2 {
		key := pairs[i]
		desc := pairs[i+1]

		if i > 0 {
			result += " " + styleHintSeparator.Render("•") + " "
		}

		result += styleHintKey.Render(key) + " " + styleHintDesc.Render(desc)
	}

	return result
}
