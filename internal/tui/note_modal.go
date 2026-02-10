package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
	"github.com/mark3labs/iteratr/internal/tui/theme"
)

// noteModalFocus tracks which element has keyboard focus in the NoteModal.
type noteModalFocus int

const (
	noteModalFocusType    noteModalFocus = iota // Type selector row
	noteModalFocusContent                       // Content textarea
	noteModalFocusDelete                        // Delete button
)

// charLimitNoteContent is the max character limit for note content editing.
const charLimitNoteContent = 500

// Valid note types in cycling order.
var noteTypes = []struct {
	value string
	icon  string
}{
	{"learning", "*"},
	{"stuck", "!"},
	{"tip", "›"},
	{"decision", "◇"},
}

// NoteModal displays detailed information about a single note in a centered overlay.
// Supports interactive type cycling, content editing, and note deletion.
type NoteModal struct {
	note    *session.Note
	visible bool
	width   int // Modal width
	height  int // Modal height
	focus   noteModalFocus

	// Current editable values (track independently so cycling is immediate)
	typeIndex int // Index into noteTypes

	// Content editing
	textarea        textarea.Model
	contentModified bool // True if textarea content differs from note.Content
}

// NewNoteModal creates a new NoteModal component.
func NewNoteModal() *NoteModal {
	ta := textarea.New()
	ta.Placeholder = "Note content..."
	ta.CharLimit = charLimitNoteContent
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.SetWidth(50)
	ta.SetHeight(4)

	// Override textarea KeyMap to remove ctrl+t from LineNext
	ta.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))

	// Style textarea
	t := theme.Current()
	styles := textarea.DefaultDarkStyles()
	styles.Cursor.Color = lipgloss.Color(t.Secondary)
	styles.Cursor.Shape = tea.CursorBlock
	styles.Cursor.Blink = true
	ta.SetStyles(styles)

	return &NoteModal{
		visible:  false,
		width:    60,
		height:   22,
		textarea: ta,
	}
}

// SetNote sets the note to display in the modal and shows it.
func (m *NoteModal) SetNote(note *session.Note) {
	m.note = note
	m.visible = true
	m.focus = noteModalFocusType
	m.contentModified = false

	// Initialize editable values from note
	m.typeIndex = noteTypeToIndex(note.Type)

	// Initialize textarea with note content
	m.textarea.SetValue(note.Content)
	m.textarea.Blur()
}

// Close hides the modal.
func (m *NoteModal) Close() {
	m.visible = false
	m.note = nil
	m.contentModified = false
	m.textarea.Blur()
}

// IsVisible returns whether the modal is currently visible.
func (m *NoteModal) IsVisible() bool {
	return m.visible
}

// Note returns the currently displayed note.
func (m *NoteModal) Note() *session.Note {
	return m.note
}

// Update handles keyboard and paste input for the interactive note modal.
func (m *NoteModal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible || m.note == nil {
		return nil
	}

	// Handle paste messages when textarea is focused
	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		if m.focus == noteModalFocusContent {
			return m.handlePaste(pasteMsg)
		}
		return nil
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Forward non-key messages to textarea when focused (cursor blink, etc.)
		if m.focus == noteModalFocusContent {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			m.checkContentModified()
			return cmd
		}
		return nil
	}

	switch keyMsg.String() {
	case "esc":
		// If textarea is focused, ESC blurs textarea first
		if m.focus == noteModalFocusContent {
			m.focus = noteModalFocusType
			m.textarea.Blur()
			return nil
		}
		m.Close()
		return nil

	case "ctrl+enter":
		// Save content if modified
		if m.contentModified {
			return m.emitContentChange()
		}
		return nil

	case "tab":
		return m.handleTab(false)

	case "shift+tab":
		return m.handleTab(true)

	case "left", "h":
		if m.focus == noteModalFocusContent {
			// Let textarea handle left arrow
			break
		}
		if m.focus == noteModalFocusType {
			m.cycleTypeBackward()
			return m.emitTypeChange()
		}
		return nil

	case "right", "l":
		if m.focus == noteModalFocusContent {
			// Let textarea handle right arrow
			break
		}
		if m.focus == noteModalFocusType {
			m.cycleTypeForward()
			return m.emitTypeChange()
		}
		return nil

	case "enter", " ":
		if m.focus == noteModalFocusDelete {
			noteID := m.note.ID
			return func() tea.Msg {
				return RequestDeleteNoteMsg{ID: noteID}
			}
		}
		// Don't intercept enter/space when textarea is focused
		if m.focus == noteModalFocusContent {
			break
		}
		return nil

	case "d":
		// Shortcut: 'd' for delete only when NOT in textarea
		if m.focus != noteModalFocusContent {
			noteID := m.note.ID
			return func() tea.Msg {
				return RequestDeleteNoteMsg{ID: noteID}
			}
		}
	}

	// Forward to textarea when it's focused
	if m.focus == noteModalFocusContent {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.checkContentModified()
		return cmd
	}

	return nil
}

// RequestDeleteNoteMsg is sent when the user requests note deletion.
// The App will show a confirmation dialog before actually deleting.
type RequestDeleteNoteMsg struct {
	ID string
}

// handleTab manages focus cycling with textarea focus/blur.
func (m *NoteModal) handleTab(backward bool) tea.Cmd {
	oldFocus := m.focus

	if backward {
		switch m.focus {
		case noteModalFocusType:
			m.focus = noteModalFocusDelete
		case noteModalFocusContent:
			m.focus = noteModalFocusType
		case noteModalFocusDelete:
			m.focus = noteModalFocusContent
		}
	} else {
		switch m.focus {
		case noteModalFocusType:
			m.focus = noteModalFocusContent
		case noteModalFocusContent:
			m.focus = noteModalFocusDelete
		case noteModalFocusDelete:
			m.focus = noteModalFocusType
		}
	}

	return m.updateTextareaFocus(oldFocus)
}

// updateTextareaFocus manages textarea focus/blur transitions.
func (m *NoteModal) updateTextareaFocus(oldFocus noteModalFocus) tea.Cmd {
	if m.focus == noteModalFocusContent && oldFocus != noteModalFocusContent {
		return m.textarea.Focus()
	}
	if m.focus != noteModalFocusContent && oldFocus == noteModalFocusContent {
		m.textarea.Blur()
	}
	return nil
}

// checkContentModified updates the contentModified flag.
func (m *NoteModal) checkContentModified() {
	if m.note == nil {
		return
	}
	m.contentModified = strings.TrimSpace(m.textarea.Value()) != strings.TrimSpace(m.note.Content)
}

// handlePaste processes paste input for the textarea with char limit enforcement.
func (m *NoteModal) handlePaste(msg tea.PasteMsg) tea.Cmd {
	currentLen := len([]rune(m.textarea.Value()))
	pasteLen := len([]rune(msg.Content))
	remainingSpace := charLimitNoteContent - currentLen

	if remainingSpace <= 0 {
		return func() tea.Msg {
			return ShowToastMsg{Text: fmt.Sprintf("%d chars truncated", pasteLen)}
		}
	}

	if pasteLen > remainingSpace {
		truncatedContent := string([]rune(msg.Content)[:remainingSpace])
		truncatedCount := pasteLen - remainingSpace
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(tea.PasteMsg{Content: truncatedContent})
		m.checkContentModified()
		return tea.Batch(cmd, func() tea.Msg {
			return ShowToastMsg{Text: fmt.Sprintf("%d chars truncated", truncatedCount)}
		})
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(tea.PasteMsg{Content: msg.Content})
	m.checkContentModified()
	return cmd
}

// cycleTypeForward cycles to the next type.
func (m *NoteModal) cycleTypeForward() {
	m.typeIndex = (m.typeIndex + 1) % len(noteTypes)
}

// cycleTypeBackward cycles to the previous type.
func (m *NoteModal) cycleTypeBackward() {
	m.typeIndex = (m.typeIndex - 1 + len(noteTypes)) % len(noteTypes)
}

// emitTypeChange returns a command that sends an UpdateNoteTypeMsg.
func (m *NoteModal) emitTypeChange() tea.Cmd {
	noteID := m.note.ID
	noteType := noteTypes[m.typeIndex].value
	return func() tea.Msg {
		return UpdateNoteTypeMsg{ID: noteID, Type: noteType}
	}
}

// emitContentChange returns a command that sends an UpdateNoteContentMsg.
func (m *NoteModal) emitContentChange() tea.Cmd {
	noteID := m.note.ID
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return nil // Don't allow empty content
	}
	m.contentModified = false
	return func() tea.Msg {
		return UpdateNoteContentMsg{ID: noteID, Content: content}
	}
}

// noteTypeToIndex converts a type string to its index in noteTypes.
func noteTypeToIndex(noteType string) int {
	for i, t := range noteTypes {
		if t.value == noteType {
			return i
		}
	}
	return 0 // Default to "learning"
}

// Draw renders the modal centered on the screen buffer (Screen/Draw pattern).
func (m *NoteModal) Draw(scr uv.Screen, area uv.Rectangle) {
	if !m.visible || m.note == nil {
		return
	}

	// Calculate modal dimensions
	modalWidth := m.width
	modalHeight := m.height

	// Ensure modal fits on screen with margins
	if modalWidth > area.Dx()-4 {
		modalWidth = area.Dx() - 4
	}
	if modalHeight > area.Dy()-4 {
		modalHeight = area.Dy() - 4
	}

	// Ensure minimum dimensions
	if modalWidth < 30 {
		modalWidth = 30
	}
	if modalHeight < 10 {
		modalHeight = 10
	}

	// Set textarea width based on modal width
	m.textarea.SetWidth(modalWidth - 8) // Account for borders + padding

	// Build modal content
	content := m.buildContent(modalWidth - 4) // Account for padding and borders

	// Style the modal with border and background
	s := theme.Current().S()
	modalStyle := s.ModalContainer.
		Width(modalWidth).
		Height(modalHeight)

	modalContent := modalStyle.Render(content)

	// Calculate center position
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

	// Draw modal centered on screen
	modalArea := uv.Rectangle{
		Min: uv.Position{X: area.Min.X + x, Y: area.Min.Y + y},
		Max: uv.Position{X: area.Min.X + x + renderedWidth, Y: area.Min.Y + y + renderedHeight},
	}
	uv.NewStyledString(modalContent).Draw(scr, modalArea)
}

// buildContent builds the modal content string with note details and interactive controls.
func (m *NoteModal) buildContent(width int) string {
	if m.note == nil {
		return ""
	}

	t := theme.Current()
	s := t.S()
	var sections []string

	// === Title Section ===
	title := renderModalTitle("Note Details", width-2)
	sections = append(sections, title)
	sections = append(sections, "")

	// === ID Section ===
	idLine := s.ModalLabel.Render("ID: ") + s.ModalValue.Render(m.note.ID)
	sections = append(sections, idLine)
	sections = append(sections, "")

	// === Interactive Type Row ===
	typeLabel := s.ModalLabel.Render("Type:     ")
	typeBadges := m.renderTypeBadges()
	sections = append(sections, typeLabel+typeBadges)
	sections = append(sections, "")

	// === Content Textarea ===
	sections = append(sections, m.textarea.View())
	sections = append(sections, "")

	// === Timestamps Section ===
	createdLine := s.ModalLabel.Render("Created:  ") + s.ModalValue.Render(m.formatTime(m.note.CreatedAt))
	updatedLine := s.ModalLabel.Render("Updated:  ") + s.ModalValue.Render(m.formatTime(m.note.UpdatedAt))
	sections = append(sections, createdLine)
	sections = append(sections, updatedLine)
	sections = append(sections, "")

	// === Delete Button + Hint Bar ===
	deleteButton := m.renderDeleteButton()
	hintBar := m.renderHintBar()
	bottomLine := deleteButton + "  " + hintBar
	bottomText := lipgloss.NewStyle().Width(width - 2).Align(lipgloss.Center).Render(bottomLine)
	sections = append(sections, bottomText)

	return strings.Join(sections, "\n")
}

// renderTypeBadges renders all type badges with the active one highlighted.
func (m *NoteModal) renderTypeBadges() string {
	t := theme.Current()
	s := t.S()
	var badges []string

	for i, nt := range noteTypes {
		isActive := i == m.typeIndex
		text := nt.icon + " " + nt.value

		var typeBadge lipgloss.Style
		var typeColor string
		switch nt.value {
		case "learning":
			typeBadge = s.BadgeSuccess
			typeColor = t.Success
		case "stuck":
			typeBadge = s.BadgeError
			typeColor = t.Error
		case "tip":
			typeBadge = s.BadgeWarning
			typeColor = t.Warning
		case "decision":
			typeBadge = s.BadgeInfo
			typeColor = t.Secondary
		default:
			typeBadge = s.BadgeMuted
			typeColor = t.FgMuted
		}

		if isActive {
			if m.focus == noteModalFocusType {
				badge := s.Badge.
					Foreground(lipgloss.Color(t.FgBright)).
					Background(lipgloss.Color(t.Primary))
				badges = append(badges, badge.Render(text))
			} else {
				badges = append(badges, typeBadge.Render(text))
			}
		} else {
			badge := s.Badge.Foreground(lipgloss.Color(typeColor))
			badges = append(badges, badge.Render(text))
		}
	}

	return strings.Join(badges, " ")
}

// renderDeleteButton renders the delete button with focus-aware styling.
func (m *NoteModal) renderDeleteButton() string {
	t := theme.Current()
	s := t.S()

	if m.focus == noteModalFocusDelete {
		buttonStyle := s.Badge.
			Foreground(lipgloss.Color(t.FgBright)).
			Background(lipgloss.Color(t.Error))
		return buttonStyle.Render("  Delete  ")
	}

	return s.BadgeMuted.Render("  Delete  ")
}

// renderHintBar renders the keyboard shortcut hints for the modal.
func (m *NoteModal) renderHintBar() string {
	return RenderHintBar(
		KeyTab, "cycle",
		"←→", "change",
		"ctrl+enter", "save",
		KeyEsc, "close",
	)
}

// renderTypeBadge renders a styled badge for a single note type (used when unfocused active).
func (m *NoteModal) renderTypeBadge(noteType string) string {
	s := theme.Current().S()
	var badge lipgloss.Style
	var text string

	switch noteType {
	case "learning":
		badge = s.BadgeSuccess
		text = "* learning"
	case "stuck":
		badge = s.BadgeError
		text = "! stuck"
	case "tip":
		badge = s.BadgeWarning
		text = "› tip"
	case "decision":
		badge = s.BadgeInfo
		text = "◇ decision"
	default:
		badge = s.BadgeMuted
		text = "≡ " + noteType
	}

	return badge.Render(text)
}

// formatTime formats a timestamp for display.
func (m *NoteModal) formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
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
