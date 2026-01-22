package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
)

// NotesPanel displays notes grouped by type with color-coding.
type NotesPanel struct {
	viewport viewport.Model
	state    *session.State
	width    int
	height   int
	focused  bool
}

// NewNotesPanel creates a new NotesPanel component.
func NewNotesPanel() *NotesPanel {
	vp := viewport.New()
	return &NotesPanel{
		viewport: vp,
	}
}

// Draw renders the notes panel to the screen buffer.
func (n *NotesPanel) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// Draw panel border with title
	inner := DrawPanel(scr, area, "Notes", n.focused)

	// Draw viewport content
	content := n.viewport.View()
	DrawText(scr, inner, content)

	// Draw scroll indicator if content overflows
	if n.viewport.TotalLineCount() > n.viewport.Height() {
		percent := n.viewport.ScrollPercent()
		DrawScrollIndicator(scr, area, percent)
	}

	return nil
}

// Update handles messages for the notes panel.
func (n *NotesPanel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	n.viewport, cmd = n.viewport.Update(msg)
	return cmd
}

// Render returns the notes panel view as a string.
func (n *NotesPanel) Render() string {
	return n.viewport.View()
}

// renderTypeHeader renders a color-coded header for a note type.
func (n *NotesPanel) renderTypeHeader(noteType string, count int) string {
	var style lipgloss.Style
	var label string

	switch noteType {
	case "learning":
		style = styleNoteTypeLearning
		label = "LEARNING"
	case "stuck":
		style = styleNoteTypeStuck
		label = "STUCK"
	case "tip":
		style = styleNoteTypeTip
		label = "TIP"
	case "decision":
		style = styleNoteTypeDecision
		label = "DECISION"
	default:
		style = styleHighlight
		label = strings.ToUpper(noteType)
	}

	headerText := fmt.Sprintf("%s (%d)", label, count)
	return style.Render(headerText)
}

// renderNote renders a single note with iteration number.
func (n *NotesPanel) renderNote(note *session.Note) string {
	// Format iteration number
	iterStr := fmt.Sprintf("[#%d]", note.Iteration)
	iterFormatted := styleNoteIteration.Render(iterStr)

	// Format content with word wrapping if needed
	content := note.Content
	maxWidth := n.width - 10 // Reserve space for indent and iteration
	if len(content) > maxWidth {
		// Simple word wrapping - split on spaces
		words := strings.Fields(content)
		var lines []string
		var currentLine string

		for _, word := range words {
			if len(currentLine)+len(word)+1 <= maxWidth {
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
			} else {
				if currentLine != "" {
					lines = append(lines, currentLine)
				}
				currentLine = word
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}

		// Format first line with iteration number
		firstLine := fmt.Sprintf("  %s %s", iterFormatted, lines[0])
		result := []string{styleNoteContent.Render(firstLine)}

		// Format continuation lines without iteration number
		for i := 1; i < len(lines); i++ {
			contLine := fmt.Sprintf("      %s", lines[i])
			result = append(result, styleNoteContent.Render(contLine))
		}

		return strings.Join(result, "\n")
	}

	// Single line note
	noteStr := fmt.Sprintf("  %s %s", iterFormatted, content)
	return styleNoteContent.Render(noteStr)
}

// UpdateSize updates the notes panel dimensions.
func (n *NotesPanel) UpdateSize(width, height int) tea.Cmd {
	n.width = width
	n.height = height
	n.viewport.SetWidth(width - 2) // Account for border
	n.viewport.SetHeight(height - 2)
	return nil
}

// UpdateState updates the notes panel with new session state.
func (n *NotesPanel) UpdateState(state *session.State) tea.Cmd {
	n.state = state
	n.updateContent()
	return nil
}

// updateContent rebuilds the viewport content from current notes.
func (n *NotesPanel) updateContent() {
	if n.state == nil || len(n.state.Notes) == 0 {
		n.viewport.SetContent(styleEmptyState.Render("No notes recorded yet"))
		return
	}

	// Group notes by type
	notesByType := make(map[string][]*session.Note)
	for _, note := range n.state.Notes {
		notesByType[note.Type] = append(notesByType[note.Type], note)
	}

	var sections []string

	// Render notes by type in consistent order
	types := []string{"learning", "decision", "tip", "stuck"}
	for _, noteType := range types {
		notes := notesByType[noteType]
		if len(notes) == 0 {
			continue
		}

		// Render type header with color-coding
		header := n.renderTypeHeader(noteType, len(notes))
		sections = append(sections, header)

		// Render individual notes
		for _, note := range notes {
			noteStr := n.renderNote(note)
			sections = append(sections, noteStr)
		}

		// Add spacing between type groups
		sections = append(sections, "")
	}

	// Join all sections and set viewport content
	content := strings.Join(sections, "\n")
	n.viewport.SetContent(content)
}
