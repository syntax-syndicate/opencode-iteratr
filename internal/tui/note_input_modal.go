package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
)

// focusZone represents which UI element has keyboard focus within the modal.
type focusZone int

const (
	focusTypeSelector focusZone = iota // Type selector (learning/stuck/tip/decision badges)
	focusTextarea                      // Multi-line textarea for note content
	focusSubmitButton                  // Submit button
)

// NoteInputModal is an interactive modal for creating new notes.
// It displays a textarea for content input and allows the user to submit notes.
type NoteInputModal struct {
	visible   bool
	textarea  textarea.Model
	noteType  string    // Current selected type (derived from types[typeIndex])
	types     []string  // Available note types: ["learning", "stuck", "tip", "decision"]
	typeIndex int       // Current index in types array
	focus     focusZone // Which UI element currently has keyboard focus
	width     int
	height    int
}

// NewNoteInputModal creates a new NoteInputModal component.
func NewNoteInputModal() *NoteInputModal {
	// Create and configure textarea
	ta := textarea.New()
	ta.Placeholder = "Enter your note..."
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.Prompt = "" // No prompt character
	ta.SetWidth(50)
	ta.SetHeight(6)

	// Define available note types
	types := []string{"learning", "stuck", "tip", "decision"}

	return &NoteInputModal{
		visible:   false,
		textarea:  ta,
		types:     types,
		typeIndex: 0,             // Start with first type (learning)
		noteType:  types[0],      // Initialize to "learning"
		focus:     focusTextarea, // Start with textarea focused
		width:     60,
		height:    16,
	}
}

// IsVisible returns whether the modal is currently visible.
func (m *NoteInputModal) IsVisible() bool {
	return m.visible
}

// Show makes the modal visible and focuses the textarea.
func (m *NoteInputModal) Show() tea.Cmd {
	m.visible = true
	return m.textarea.Focus()
}

// Close hides the modal and resets its state.
func (m *NoteInputModal) Close() {
	m.visible = false
	m.reset()
}

// reset clears the textarea and resets the modal to initial state.
// Called on both cancel (ESC) and submit to ensure clean state on next open.
func (m *NoteInputModal) reset() {
	// Clear textarea content
	m.textarea.SetValue("")

	// Reset type selector to default (first type: learning)
	m.typeIndex = 0
	m.noteType = m.types[0]

	// Reset focus to textarea (default starting position)
	m.focus = focusTextarea

	// Blur the textarea to reset its internal state
	m.textarea.Blur()
}

// cycleFocusForward moves focus to the next element in the cycle:
// type selector → textarea → submit button → type selector (wraps)
// Returns a command to focus the textarea if it becomes the active element.
func (m *NoteInputModal) cycleFocusForward() tea.Cmd {
	oldFocus := m.focus

	switch m.focus {
	case focusTypeSelector:
		m.focus = focusTextarea
	case focusTextarea:
		m.focus = focusSubmitButton
	case focusSubmitButton:
		m.focus = focusTypeSelector
	}

	return m.updateTextareaFocus(oldFocus)
}

// cycleFocusBackward moves focus to the previous element in the cycle:
// button → textarea → type selector → button (wraps)
// Returns a command to focus the textarea if it becomes the active element.
func (m *NoteInputModal) cycleFocusBackward() tea.Cmd {
	oldFocus := m.focus

	switch m.focus {
	case focusTypeSelector:
		m.focus = focusSubmitButton
	case focusTextarea:
		m.focus = focusTypeSelector
	case focusSubmitButton:
		m.focus = focusTextarea
	}

	return m.updateTextareaFocus(oldFocus)
}

// updateTextareaFocus manages the textarea's focus/blur state based on focus changes.
// If focus moved TO textarea, it calls Focus(). If focus moved AWAY from textarea, it calls Blur().
// Returns the Focus() command if textarea should be focused, nil otherwise.
func (m *NoteInputModal) updateTextareaFocus(oldFocus focusZone) tea.Cmd {
	// Focus moved TO textarea
	if m.focus == focusTextarea && oldFocus != focusTextarea {
		return m.textarea.Focus()
	}

	// Focus moved AWAY from textarea
	if m.focus != focusTextarea && oldFocus == focusTextarea {
		m.textarea.Blur()
	}

	return nil
}

// Update handles keyboard input for the modal.
// For now, this is a minimal implementation that will be expanded in later tasks.
func (m *NoteInputModal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	// Handle key presses
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			// ESC closes the modal
			m.Close()
			return nil
		case "ctrl+enter":
			// Ctrl+Enter submits the note
			// Get the content from textarea
			content := strings.TrimSpace(m.textarea.Value())

			// Don't submit if empty (validation)
			if content == "" {
				return nil
			}

			// Return a function that creates the CreateNoteMsg
			// The iteration will be set by App when it receives this
			return m.submit(content)
		case "tab":
			// Tab cycles focus forward: type selector → textarea → button
			return m.cycleFocusForward()
		case "shift+tab":
			// Shift+Tab cycles focus backward: button → textarea → type selector
			return m.cycleFocusBackward()
		case "enter", " ":
			// Enter or Space when button is focused submits the note
			if m.focus == focusSubmitButton {
				content := strings.TrimSpace(m.textarea.Value())

				// Don't submit if empty (validation)
				if content == "" {
					return nil
				}

				return m.submit(content)
			}
		}
	}

	// Forward messages to textarea only when it's focused
	// This ensures other focus zones don't accidentally trigger textarea input
	if m.focus == focusTextarea {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return cmd
	}

	return nil
}

// submit returns a command that creates a CreateNoteMsg.
// The App will receive this message and fill in the iteration number.
func (m *NoteInputModal) submit(content string) tea.Cmd {
	noteType := m.noteType
	return func() tea.Msg {
		return CreateNoteMsg{
			Content:   content,
			NoteType:  noteType,
			Iteration: 0, // Will be filled in by App
		}
	}
}

// View renders the modal content (for testing and integration).
func (m *NoteInputModal) View() string {
	if !m.visible {
		return ""
	}

	var sections []string

	// Title
	title := renderModalTitle("New Note", m.width-4)
	sections = append(sections, title)
	sections = append(sections, "")

	// Textarea
	sections = append(sections, m.textarea.View())
	sections = append(sections, "")

	// Submit button (static, unfocused state for now)
	button := m.renderButton()
	buttonLine := lipgloss.NewStyle().Width(m.width - 4).Align(lipgloss.Right).Render(button)
	sections = append(sections, buttonLine)

	return strings.Join(sections, "\n")
}

// renderButton renders the submit button in its current state with appropriate styling.
// Three states:
// - Focused: highlighted with primary color background
// - Unfocused: muted with dim background
// - Disabled: muted and visually dimmed (when content is empty)
func (m *NoteInputModal) renderButton() string {
	content := strings.TrimSpace(m.textarea.Value())
	isEmpty := content == ""

	var buttonStyle lipgloss.Style

	// Disabled state: content is empty
	if isEmpty {
		buttonStyle = styleBadgeMuted.Copy().
			Foreground(colorSubtext0). // Dimmed text
			Background(colorSurface0)  // Very subtle background
	} else if m.focus == focusSubmitButton {
		// Focused state: highlighted with primary color
		buttonStyle = styleBadge.Copy().
			Foreground(colorTextBright). // Bright text
			Background(colorPrimary)     // Primary brand color
	} else {
		// Unfocused state: standard muted style
		buttonStyle = styleBadgeMuted.Copy()
	}

	return buttonStyle.Render("  Save Note  ")
}

// Draw renders the modal centered on the screen buffer.
func (m *NoteInputModal) Draw(scr uv.Screen, area uv.Rectangle) {
	if !m.visible {
		return
	}

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
	if modalHeight < 8 {
		modalHeight = 8
	}

	// Build modal content using View()
	content := m.View()

	// Style the modal with border and background
	modalStyle := styleModalContainer.
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
