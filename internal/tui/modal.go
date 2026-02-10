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

// taskModalFocus tracks which element has keyboard focus in the TaskModal.
type taskModalFocus int

const (
	taskModalFocusStatus   taskModalFocus = iota // Status selector row
	taskModalFocusPriority                       // Priority selector row
	taskModalFocusContent                        // Content textarea
	taskModalFocusDelete                         // Delete button
)

// charLimitTaskContent is the max character limit for task content editing.
const charLimitTaskContent = 500

// Valid task statuses in cycling order.
var taskStatuses = []struct {
	value string
	icon  string
}{
	{"remaining", "○"},
	{"in_progress", "►"},
	{"completed", "✓"},
	{"blocked", "⊘"},
	{"cancelled", "⊗"},
}

// TaskModal displays detailed information about a single task in a centered overlay.
// Supports interactive status/priority cycling, content editing, and task deletion.
type TaskModal struct {
	task    *session.Task
	visible bool
	width   int // Modal width
	height  int // Modal height
	focus   taskModalFocus

	// Current editable values (track independently so cycling is immediate)
	statusIndex   int // Index into taskStatuses
	priorityIndex int // Index into priorities (from task_input_modal.go)

	// Content editing
	textarea        textarea.Model
	contentModified bool // True if textarea content differs from task.Content
}

// NewTaskModal creates a new TaskModal component.
func NewTaskModal() *TaskModal {
	ta := textarea.New()
	ta.Placeholder = "Task description..."
	ta.CharLimit = charLimitTaskContent
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

	return &TaskModal{
		visible:  false,
		width:    60,
		height:   26, // Taller to fit textarea
		textarea: ta,
	}
}

// SetTask sets the task to display in the modal and shows it.
func (m *TaskModal) SetTask(task *session.Task) {
	m.task = task
	m.visible = true
	m.focus = taskModalFocusStatus
	m.contentModified = false

	// Initialize editable values from task
	m.statusIndex = statusToIndex(task.Status)
	m.priorityIndex = task.Priority
	if m.priorityIndex < 0 || m.priorityIndex > 4 {
		m.priorityIndex = 2
	}

	// Initialize textarea with task content
	m.textarea.SetValue(task.Content)
	m.textarea.Blur()
}

// Close hides the modal.
func (m *TaskModal) Close() {
	m.visible = false
	m.task = nil
	m.contentModified = false
	m.textarea.Blur()
}

// IsVisible returns whether the modal is currently visible.
func (m *TaskModal) IsVisible() bool {
	return m.visible
}

// Task returns the currently displayed task.
func (m *TaskModal) Task() *session.Task {
	return m.task
}

// Update handles keyboard and paste input for the interactive task modal.
func (m *TaskModal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible || m.task == nil {
		return nil
	}

	// Handle paste messages when textarea is focused
	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		if m.focus == taskModalFocusContent {
			return m.handlePaste(pasteMsg)
		}
		return nil
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Forward non-key messages to textarea when focused (cursor blink, etc.)
		if m.focus == taskModalFocusContent {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			m.checkContentModified()
			return cmd
		}
		return nil
	}

	switch keyMsg.String() {
	case "esc":
		// If content was modified and textarea is focused, ESC blurs textarea first
		if m.focus == taskModalFocusContent {
			m.focus = taskModalFocusStatus
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
		if m.focus == taskModalFocusContent {
			// Let textarea handle left arrow
			break
		}
		switch m.focus {
		case taskModalFocusStatus:
			m.cycleStatusBackward()
			return m.emitStatusChange()
		case taskModalFocusPriority:
			m.cyclePriorityBackward()
			return m.emitPriorityChange()
		}
		return nil

	case "right", "l":
		if m.focus == taskModalFocusContent {
			// Let textarea handle right arrow
			break
		}
		switch m.focus {
		case taskModalFocusStatus:
			m.cycleStatusForward()
			return m.emitStatusChange()
		case taskModalFocusPriority:
			m.cyclePriorityForward()
			return m.emitPriorityChange()
		}
		return nil

	case "enter", " ":
		if m.focus == taskModalFocusDelete {
			taskID := m.task.ID
			return func() tea.Msg {
				return RequestDeleteTaskMsg{ID: taskID}
			}
		}
		// Don't intercept enter/space when textarea is focused
		if m.focus == taskModalFocusContent {
			break
		}
		return nil

	case "d":
		// Shortcut: 'd' for delete only when NOT in textarea
		if m.focus != taskModalFocusContent {
			taskID := m.task.ID
			return func() tea.Msg {
				return RequestDeleteTaskMsg{ID: taskID}
			}
		}
	}

	// Forward to textarea when it's focused
	if m.focus == taskModalFocusContent {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.checkContentModified()
		return cmd
	}

	return nil
}

// RequestDeleteTaskMsg is sent when the user requests task deletion.
// The App will show a confirmation dialog before actually deleting.
type RequestDeleteTaskMsg struct {
	ID string
}

// handleTab manages focus cycling with textarea focus/blur.
func (m *TaskModal) handleTab(backward bool) tea.Cmd {
	oldFocus := m.focus

	if backward {
		switch m.focus {
		case taskModalFocusStatus:
			m.focus = taskModalFocusDelete
		case taskModalFocusPriority:
			m.focus = taskModalFocusStatus
		case taskModalFocusContent:
			m.focus = taskModalFocusPriority
		case taskModalFocusDelete:
			m.focus = taskModalFocusContent
		}
	} else {
		switch m.focus {
		case taskModalFocusStatus:
			m.focus = taskModalFocusPriority
		case taskModalFocusPriority:
			m.focus = taskModalFocusContent
		case taskModalFocusContent:
			m.focus = taskModalFocusDelete
		case taskModalFocusDelete:
			m.focus = taskModalFocusStatus
		}
	}

	return m.updateTextareaFocus(oldFocus)
}

// updateTextareaFocus manages textarea focus/blur transitions.
func (m *TaskModal) updateTextareaFocus(oldFocus taskModalFocus) tea.Cmd {
	if m.focus == taskModalFocusContent && oldFocus != taskModalFocusContent {
		return m.textarea.Focus()
	}
	if m.focus != taskModalFocusContent && oldFocus == taskModalFocusContent {
		m.textarea.Blur()
	}
	return nil
}

// checkContentModified updates the contentModified flag.
func (m *TaskModal) checkContentModified() {
	if m.task == nil {
		return
	}
	m.contentModified = strings.TrimSpace(m.textarea.Value()) != strings.TrimSpace(m.task.Content)
}

// handlePaste processes paste input for the textarea with char limit enforcement.
func (m *TaskModal) handlePaste(msg tea.PasteMsg) tea.Cmd {
	currentLen := len([]rune(m.textarea.Value()))
	pasteLen := len([]rune(msg.Content))
	remainingSpace := charLimitTaskContent - currentLen

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

// cycleStatusForward cycles to the next status.
func (m *TaskModal) cycleStatusForward() {
	m.statusIndex = (m.statusIndex + 1) % len(taskStatuses)
}

// cycleStatusBackward cycles to the previous status.
func (m *TaskModal) cycleStatusBackward() {
	m.statusIndex = (m.statusIndex - 1 + len(taskStatuses)) % len(taskStatuses)
}

// cyclePriorityForward cycles to the next priority.
func (m *TaskModal) cyclePriorityForward() {
	m.priorityIndex = (m.priorityIndex + 1) % len(priorities)
}

// cyclePriorityBackward cycles to the previous priority.
func (m *TaskModal) cyclePriorityBackward() {
	m.priorityIndex = (m.priorityIndex - 1 + len(priorities)) % len(priorities)
}

// emitStatusChange returns a command that sends an UpdateTaskStatusMsg.
func (m *TaskModal) emitStatusChange() tea.Cmd {
	taskID := m.task.ID
	status := taskStatuses[m.statusIndex].value
	return func() tea.Msg {
		return UpdateTaskStatusMsg{ID: taskID, Status: status}
	}
}

// emitPriorityChange returns a command that sends an UpdateTaskPriorityMsg.
func (m *TaskModal) emitPriorityChange() tea.Cmd {
	taskID := m.task.ID
	priority := priorities[m.priorityIndex].value
	return func() tea.Msg {
		return UpdateTaskPriorityMsg{ID: taskID, Priority: priority}
	}
}

// emitContentChange returns a command that sends an UpdateTaskContentMsg.
func (m *TaskModal) emitContentChange() tea.Cmd {
	taskID := m.task.ID
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return nil // Don't allow empty content
	}
	m.contentModified = false
	return func() tea.Msg {
		return UpdateTaskContentMsg{ID: taskID, Content: content}
	}
}

// statusToIndex converts a status string to its index in taskStatuses.
func statusToIndex(status string) int {
	for i, s := range taskStatuses {
		if s.value == status {
			return i
		}
	}
	return 0 // Default to "remaining"
}

// Draw renders the modal centered on the screen buffer (Screen/Draw pattern).
func (m *TaskModal) Draw(scr uv.Screen, area uv.Rectangle) {
	if !m.visible || m.task == nil {
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

// buildContent builds the modal content string with task details and interactive controls.
func (m *TaskModal) buildContent(width int) string {
	if m.task == nil {
		return ""
	}

	t := theme.Current()
	s := t.S()
	var sections []string

	// === Title Section ===
	title := renderModalTitle("Task Details", width-2)
	sections = append(sections, title)
	sections = append(sections, "")

	// === ID Section ===
	idLine := s.ModalLabel.Render("ID: ") + s.ModalValue.Render(m.task.ID)
	sections = append(sections, idLine)
	sections = append(sections, "")

	// === Interactive Status Row ===
	statusLabel := s.ModalLabel.Render("Status:   ")
	statusBadges := m.renderStatusBadges()
	sections = append(sections, statusLabel+statusBadges)
	sections = append(sections, "")

	// === Interactive Priority Row ===
	priorityLabel := s.ModalLabel.Render("Priority: ")
	priorityBadges := m.renderPriorityBadges()
	sections = append(sections, priorityLabel+priorityBadges)
	sections = append(sections, "")

	// === Content Textarea ===
	sections = append(sections, m.textarea.View())
	sections = append(sections, "")

	// === Dependencies Section ===
	if len(m.task.DependsOn) > 0 {
		depsLabel := s.ModalLabel.Render("Depends on: ")
		depsContent := s.ModalValue.Render(strings.Join(m.task.DependsOn, ", "))
		sections = append(sections, depsLabel+depsContent)
	}

	// === Timestamps Section ===
	createdLine := s.ModalLabel.Render("Created:  ") + s.ModalValue.Render(m.formatTime(m.task.CreatedAt))
	updatedLine := s.ModalLabel.Render("Updated:  ") + s.ModalValue.Render(m.formatTime(m.task.UpdatedAt))
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

// renderStatusBadges renders all status badges with the active one highlighted.
func (m *TaskModal) renderStatusBadges() string {
	t := theme.Current()
	s := t.S()
	var badges []string

	for i, status := range taskStatuses {
		isActive := i == m.statusIndex
		text := status.icon + " " + status.value

		if isActive {
			if m.focus == taskModalFocusStatus {
				badge := s.Badge.
					Foreground(lipgloss.Color(t.FgBright)).
					Background(lipgloss.Color(t.Primary))
				badges = append(badges, badge.Render(text))
			} else {
				badges = append(badges, m.renderStatusBadge(status.value))
			}
		} else {
			badge := s.Badge.Foreground(lipgloss.Color(t.FgMuted))
			badges = append(badges, badge.Render(text))
		}
	}

	return strings.Join(badges, " ")
}

// renderPriorityBadges renders all priority badges with the active one highlighted.
func (m *TaskModal) renderPriorityBadges() string {
	t := theme.Current()
	s := t.S()
	var badges []string

	for i, priority := range priorities {
		isActive := i == m.priorityIndex
		text := priority.icon + " " + priority.label

		var priorityBadge lipgloss.Style
		var priorityColor string
		switch priority.value {
		case 0:
			priorityBadge = s.BadgeError
			priorityColor = t.Error
		case 1:
			priorityBadge = s.BadgeWarning
			priorityColor = t.Warning
		case 2:
			priorityBadge = s.BadgeInfo
			priorityColor = t.Secondary
		case 3:
			priorityBadge = s.BadgeMuted
			priorityColor = t.FgMuted
		case 4:
			priorityBadge = s.BadgeMuted.Faint(true)
			priorityColor = t.FgMuted
		default:
			priorityBadge = s.BadgeMuted
			priorityColor = t.FgMuted
		}

		if isActive {
			if m.focus == taskModalFocusPriority {
				badge := s.Badge.
					Foreground(lipgloss.Color(t.FgBright)).
					Background(lipgloss.Color(t.Primary))
				badges = append(badges, badge.Render(text))
			} else {
				badges = append(badges, priorityBadge.Render(text))
			}
		} else {
			badge := s.Badge.Foreground(lipgloss.Color(priorityColor))
			badges = append(badges, badge.Render(text))
		}
	}

	return strings.Join(badges, " ")
}

// renderDeleteButton renders the delete button with focus-aware styling.
func (m *TaskModal) renderDeleteButton() string {
	t := theme.Current()
	s := t.S()

	if m.focus == taskModalFocusDelete {
		buttonStyle := s.Badge.
			Foreground(lipgloss.Color(t.FgBright)).
			Background(lipgloss.Color(t.Error))
		return buttonStyle.Render("  Delete  ")
	}

	return s.BadgeMuted.Render("  Delete  ")
}

// renderHintBar renders the keyboard shortcut hints for the modal.
func (m *TaskModal) renderHintBar() string {
	return RenderHintBar(
		KeyTab, "cycle",
		"←→", "change",
		"ctrl+enter", "save",
		KeyEsc, "close",
	)
}

// renderStatusBadge renders a styled badge for a single task status.
func (m *TaskModal) renderStatusBadge(status string) string {
	s := theme.Current().S()
	var badge lipgloss.Style
	var text string

	switch status {
	case "in_progress":
		badge = s.BadgeWarning
		text = "► in_progress"
	case "remaining":
		badge = s.BadgeMuted
		text = "○ remaining"
	case "completed":
		badge = s.BadgeSuccess
		text = "✓ completed"
	case "blocked":
		badge = s.BadgeError
		text = "⊘ blocked"
	case "cancelled":
		badge = s.BadgeError
		text = "⊗ cancelled"
	default:
		badge = s.BadgeMuted
		text = "○ " + status
	}

	return badge.Render(text)
}

// formatTime formats a timestamp for display.
func (m *TaskModal) formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// wordWrap wraps text to fit within the specified width.
func (m *TaskModal) wordWrap(text string, width int) string {
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
