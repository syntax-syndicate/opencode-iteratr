package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
)

// focusPrioritySelector is the focusZone value for the priority selector in TaskInputModal.
// We reuse the focusTypeSelector value since they serve the same role (first selector in modal).
const focusPrioritySelector = focusTypeSelector

// Priority levels matching session.Task priority values.
// The value field maps to the integer stored in the Task struct and event metadata.
var priorities = []struct {
	value int
	label string
	emoji string
}{
	{0, "critical", "🔴"},
	{1, "high", "🟠"},
	{2, "medium", "🟡"},
	{3, "low", "🟢"},
	{4, "backlog", "⚪"},
}

// TaskInputModal is an interactive modal for creating new tasks.
// It displays a textarea for content input, a priority selector, and allows the user to submit tasks.
type TaskInputModal struct {
	visible       bool
	textarea      textarea.Model // Bubbles v2 textarea
	priorityIndex int            // Current selected priority (0-4)
	focus         focusZone      // Which UI element currently has keyboard focus
	width         int
	height        int
	buttonArea    uv.Rectangle // Hit area for mouse click on submit button
}

// NewTaskInputModal creates a new TaskInputModal component.
func NewTaskInputModal() *TaskInputModal {
	// Create and configure textarea
	ta := textarea.New()
	ta.Placeholder = "Describe the task..."
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.Prompt = "" // No prompt character
	ta.SetWidth(50)
	ta.SetHeight(6)

	// Override textarea KeyMap to remove ctrl+t from LineNext
	// By default, textarea binds ctrl+t to move cursor down (LineNext)
	// We only want the down arrow key for this action, not ctrl+t
	// This prevents confusion since ctrl+t opens the task modal globally
	ta.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))

	// Style textarea to match modal theme using default dark styles
	// and customizing the cursor color to match our secondary brand color
	styles := textarea.DefaultDarkStyles()
	styles.Cursor.Color = lipgloss.Color(colorSecondary)
	styles.Cursor.Shape = tea.CursorBlock
	styles.Cursor.Blink = true
	ta.SetStyles(styles)

	return &TaskInputModal{
		visible:       false,
		textarea:      ta,
		priorityIndex: 2,             // Default to medium
		focus:         focusTextarea, // Start with textarea focused
		width:         60,
		height:        18, // Slightly taller than note modal to fit priority row
	}
}

// IsVisible returns whether the modal is currently visible.
func (m *TaskInputModal) IsVisible() bool {
	return m.visible
}

// Show makes the modal visible and focuses the textarea.
func (m *TaskInputModal) Show() tea.Cmd {
	m.visible = true
	m.focus = focusTextarea
	return m.textarea.Focus()
}

// Close hides the modal and resets its state.
func (m *TaskInputModal) Close() {
	m.visible = false
	m.reset()
}

// reset clears the textarea and resets the modal to initial state.
// Called on both cancel (ESC) and submit to ensure clean state on next open.
func (m *TaskInputModal) reset() {
	// Clear textarea content
	m.textarea.SetValue("")

	// Reset priority to default (medium)
	m.priorityIndex = 2

	// Reset focus to textarea (default starting position)
	m.focus = focusTextarea

	// Blur the textarea to reset its internal state
	m.textarea.Blur()
}

// Update handles keyboard input for the modal.
// For now, this is a minimal implementation that handles ESC to close.
// Will be expanded in later tasks to handle all keyboard interactions.
func (m *TaskInputModal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	// Handle key presses
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			// ESC closes the modal without saving
			m.Close()
			return nil
		case "ctrl+enter":
			// Ctrl+Enter submits the task from any focus zone
			// Get the content from textarea
			content := strings.TrimSpace(m.textarea.Value())

			// Don't submit if empty (validation)
			if content == "" {
				return nil
			}

			// Return a function that creates the CreateTaskMsg
			// The iteration will be set by App when it receives this
			return m.submit(content)
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

// submit returns a command that creates a CreateTaskMsg.
// The App will receive this message and fill in the iteration number.
func (m *TaskInputModal) submit(content string) tea.Cmd {
	priority := priorities[m.priorityIndex].value
	return func() tea.Msg {
		return CreateTaskMsg{
			Content:   content,
			Priority:  priority,
			Iteration: 0, // Will be filled in by App
		}
	}
}

// View renders the modal content (for testing and integration).
// Returns the modal content as a string that will be styled by Draw().
func (m *TaskInputModal) View() string {
	if !m.visible {
		return ""
	}

	var sections []string

	// Title
	title := renderModalTitle("New Task", m.width-4)
	sections = append(sections, title)
	sections = append(sections, "")

	// Textarea (priority badges and button will be added in later tasks)
	sections = append(sections, m.textarea.View())
	sections = append(sections, "")

	// Hint bar placeholder (will be populated in later task)
	sections = append(sections, "")

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Draw renders the modal centered on the screen buffer.
func (m *TaskInputModal) Draw(scr uv.Screen, area uv.Rectangle) {
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
