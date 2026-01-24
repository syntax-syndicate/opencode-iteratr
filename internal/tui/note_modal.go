package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
)

// NoteModal displays detailed information about a single note in a centered overlay.
type NoteModal struct {
	note    *session.Note
	visible bool
	width   int
	height  int
}

// NewNoteModal creates a new NoteModal component.
func NewNoteModal() *NoteModal {
	return &NoteModal{
		visible: false,
		width:   60,
		height:  14,
	}
}

// SetNote sets the note to display in the modal and shows it.
func (m *NoteModal) SetNote(note *session.Note) {
	m.note = note
	m.visible = true
}

// Close hides the modal.
func (m *NoteModal) Close() {
	m.visible = false
	m.note = nil
}

// IsVisible returns whether the modal is currently visible.
func (m *NoteModal) IsVisible() bool {
	return m.visible
}

// Draw renders the modal centered on the screen buffer.
func (m *NoteModal) Draw(scr uv.Screen, area uv.Rectangle) {
	if !m.visible || m.note == nil {
		return
	}

	modalWidth := m.width
	modalHeight := m.height

	if modalWidth > area.Dx()-4 {
		modalWidth = area.Dx() - 4
	}
	if modalHeight > area.Dy()-4 {
		modalHeight = area.Dy() - 4
	}
	if modalWidth < 30 {
		modalWidth = 30
	}
	if modalHeight < 8 {
		modalHeight = 8
	}

	content := m.buildContent(modalWidth - 4)

	modalStyle := styleModalContainer.
		Width(modalWidth).
		Height(modalHeight)

	modalContent := modalStyle.Render(content)

	renderedWidth := lipgloss.Width(modalContent)
	renderedHeight := lipgloss.Height(modalContent)
	x := (area.Dx() - renderedWidth) / 2
	y := (area.Dy() - renderedHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	modalArea := uv.Rectangle{
		Min: uv.Position{X: area.Min.X + x, Y: area.Min.Y + y},
		Max: uv.Position{X: area.Min.X + x + renderedWidth, Y: area.Min.Y + y + renderedHeight},
	}
	uv.NewStyledString(modalContent).Draw(scr, modalArea)
}

// buildContent builds the modal content string with note details.
func (m *NoteModal) buildContent(width int) string {
	if m.note == nil {
		return ""
	}

	var sections []string

	// Title (with diagonal hatching decoration)
	title := renderModalTitle("Note Details", width-2)
	sections = append(sections, title)
	sections = append(sections, "")

	// ID
	idLine := styleModalLabel.Render("ID: ") + styleModalValue.Render(m.note.ID)
	sections = append(sections, idLine)
	sections = append(sections, "")

	// Type badge
	typeBadge := m.renderTypeBadge(m.note.Type)
	typeLine := styleModalLabel.Render("Type: ") + typeBadge
	sections = append(sections, typeLine)
	sections = append(sections, "")

	// Separator
	separator := styleModalSeparator.Render(strings.Repeat("─", width-2))
	sections = append(sections, separator)
	sections = append(sections, "")

	// Content (word-wrapped)
	wrappedContent := styleModalSection.Render(m.wordWrap(m.note.Content, width-2))
	sections = append(sections, wrappedContent)
	sections = append(sections, "")

	// Separator
	sections = append(sections, separator)
	sections = append(sections, "")

	// Timestamp
	createdLine := styleModalLabel.Render("Created:  ") + styleModalValue.Render(m.note.CreatedAt.Format("2006-01-02 15:04:05"))
	sections = append(sections, createdLine)
	sections = append(sections, "")

	// Close instructions (key/description differentiation)
	closeHint := styleHintKey.Render("esc") + " " +
		styleHintDesc.Render("close") + " " +
		styleHintSeparator.Render("•") + " " +
		styleHintKey.Render("click outside") + " " +
		styleHintDesc.Render("dismiss")
	closeText := lipgloss.NewStyle().Width(width - 2).Align(lipgloss.Center).Render(closeHint)
	sections = append(sections, closeText)

	return strings.Join(sections, "\n")
}

// renderTypeBadge renders a styled badge for the note type.
func (m *NoteModal) renderTypeBadge(noteType string) string {
	var badge lipgloss.Style
	var text string

	switch noteType {
	case "learning":
		badge = styleBadgeSuccess
		text = "💡 learning"
	case "stuck":
		badge = styleBadgeError
		text = "🚫 stuck"
	case "tip":
		badge = styleBadgeWarning
		text = "💬 tip"
	case "decision":
		badge = styleBadgeInfo
		text = "⚡ decision"
	default:
		badge = styleBadgeMuted
		text = "📝 " + noteType
	}

	return badge.Render(text)
}

// wordWrap wraps text to fit within the specified width.
func (m *NoteModal) wordWrap(text string, width int) string {
	if width <= 0 {
		width = 40
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		} else {
			currentLine = testLine
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

// Update handles messages for the modal.
func (m *NoteModal) Update(msg tea.Msg) tea.Cmd {
	return nil
}
