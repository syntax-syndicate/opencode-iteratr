package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
)

// OpenTaskModalMsg is sent when a task should be opened in the modal.
type OpenTaskModalMsg struct {
	Task *session.Task
}

// Sidebar displays tasks and notes in two sections.
type Sidebar struct {
	state            *session.State
	width            int
	height           int
	tasksViewport    viewport.Model
	notesViewport    viewport.Model
	cursor           int    // Selected task index (for keyboard navigation)
	activeTaskID     string // Currently active task (shown in modal)
	focused          bool
	taskIndex        map[string]int // O(1) lookup: task ID -> index in ordered task list
	noteIndex        map[string]int // O(1) lookup: note ID -> index in state.Notes
	pulse            Pulse
	pulsedTaskIDs    map[string]string // Track task ID -> last status to detect changes
	needsPulse       bool              // Flag indicating pulse should start on next Update
	tasksContentArea uv.Rectangle      // Screen area where task lines are drawn (for mouse hit detection)
	notesContentArea uv.Rectangle      // Screen area where note lines are drawn (for mouse hit detection)
	activeNoteID     string            // Currently active note (shown in modal)
}

// NewSidebar creates a new Sidebar component.
func NewSidebar() *Sidebar {
	return &Sidebar{
		tasksViewport: viewport.New(),
		notesViewport: viewport.New(),
		cursor:        0,
		focused:       false,
		taskIndex:     make(map[string]int),
		noteIndex:     make(map[string]int),
		pulse:         NewPulse(),
		pulsedTaskIDs: make(map[string]string),
	}
}

// Update handles messages for the sidebar.
func (s *Sidebar) Update(msg tea.Msg) tea.Cmd {
	// Start pulse if needed
	if s.needsPulse && !s.pulse.IsActive() {
		s.needsPulse = false
		return s.pulse.Start()
	}

	switch msg := msg.(type) {
	case PulseMsg:
		// Handle pulse animation (even when not focused)
		return s.pulse.Update(msg)
	case tea.KeyPressMsg:
		if !s.focused {
			return nil
		}
		return s.handleKeyPress(msg)
	}
	return nil
}

// handleKeyPress handles keyboard input for task navigation and viewport scrolling.
func (s *Sidebar) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	tasks := s.getTasks()

	switch msg.String() {
	case "j", "down":
		// Move cursor down
		if len(tasks) > 0 && s.cursor < len(tasks)-1 {
			s.cursor++
			s.updateContent() // Rebuild content with new cursor position
		}
		return nil
	case "k", "up":
		// Move cursor up
		if s.cursor > 0 {
			s.cursor--
			s.updateContent() // Rebuild content with new cursor position
		}
		return nil
	case "enter":
		// Return OpenTaskModalMsg for the selected task
		if len(tasks) > 0 && s.cursor < len(tasks) {
			return func() tea.Msg {
				return OpenTaskModalMsg{Task: tasks[s.cursor]}
			}
		}
		return nil
	}

	// Delegate to viewports for scrolling (pgup/pgdown, etc.)
	var cmds []tea.Cmd

	var cmd tea.Cmd
	s.tasksViewport, cmd = s.tasksViewport.Update(msg)
	cmds = append(cmds, cmd)

	s.notesViewport, cmd = s.notesViewport.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// drawTasksSection renders the tasks section with header and viewport content.
func (s *Sidebar) drawTasksSection(scr uv.Screen, area uv.Rectangle) {
	// Apply pulse effect to title if task status changed
	title := "Tasks"
	if s.pulse.IsActive() {
		// Add visual indicator when pulse is active
		intensity := s.pulse.Intensity()
		if intensity > 0.5 {
			title = "Tasks ●" // Add dot indicator during pulse
		}
	}

	// Draw panel with "Tasks" title
	inner := DrawPanel(scr, area, title, s.focused)

	// Store the inner content area for coordinate-based mouse hit detection
	s.tasksContentArea = inner

	// Render viewport content
	content := s.tasksViewport.View()
	DrawText(scr, inner, content)

	// Draw scroll indicator if needed
	if s.tasksViewport.TotalLineCount() > s.tasksViewport.Height() {
		pct := s.tasksViewport.ScrollPercent()
		indicator := fmt.Sprintf(" %d%% ", int(pct*100))
		indicatorArea := uv.Rect(
			area.Max.X-len(indicator)-1,
			area.Max.Y-1,
			len(indicator),
			1,
		)
		DrawStyled(scr, indicatorArea, styleScrollIndicator, indicator)
	}
}

// drawNotesSection renders the notes section with header and viewport content.
func (s *Sidebar) drawNotesSection(scr uv.Screen, area uv.Rectangle) {
	// Draw panel with "Notes" title
	inner := DrawPanel(scr, area, "Notes", false) // Notes section never focused

	// Store the inner content area for coordinate-based mouse hit detection
	s.notesContentArea = inner

	// Render viewport content
	content := s.notesViewport.View()
	DrawText(scr, inner, content)

	// Draw scroll indicator if needed
	if s.notesViewport.TotalLineCount() > s.notesViewport.Height() {
		pct := s.notesViewport.ScrollPercent()
		indicator := fmt.Sprintf(" %d%% ", int(pct*100))
		indicatorArea := uv.Rect(
			area.Max.X-len(indicator)-1,
			area.Max.Y-1,
			len(indicator),
			1,
		)
		DrawStyled(scr, indicatorArea, styleScrollIndicator, indicator)
	}
}

// getTasks returns all tasks ordered by ID.
func (s *Sidebar) getTasks() []*session.Task {
	if s.state == nil {
		return nil
	}

	tasks := make([]*session.Task, 0, len(s.state.Tasks))
	for _, task := range s.state.Tasks {
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// Draw renders the sidebar to the screen buffer with tasks and notes sections.
func (s *Sidebar) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// Guard against zero dimensions
	if area.Dx() < 10 || area.Dy() < 5 {
		return nil
	}

	// Split area vertically: Tasks (60%) | Notes (40%)
	tasksHeight := int(float64(area.Dy()) * 0.6)
	if tasksHeight < 3 {
		tasksHeight = 3
	}

	tasksArea, notesArea := uv.SplitVertical(area, uv.Fixed(tasksHeight))

	// Draw Tasks section
	s.drawTasksSection(scr, tasksArea)

	// Draw Notes section
	s.drawNotesSection(scr, notesArea)

	return nil
}

// buildTasksContent builds the content string for the tasks viewport.
func (s *Sidebar) buildTasksContent() string {
	tasks := s.getTasks()
	if len(tasks) == 0 {
		return styleDim.Render("  No tasks")
	}

	var lines []string
	for idx, task := range tasks {
		isSelected := (s.focused && idx == s.cursor) || task.ID == s.activeTaskID
		line := s.renderTask(task, isSelected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderTask renders a single task line with optional highlight for selected task.
func (s *Sidebar) renderTask(task *session.Task, isSelected bool) string {
	// Status indicator
	var indicator string
	var indicatorStyle lipgloss.Style

	switch task.Status {
	case "in_progress":
		indicator = "►"
		indicatorStyle = styleStatusInProgress
	case "remaining":
		indicator = "○"
		indicatorStyle = styleStatusRemaining
	case "completed":
		indicator = "✓"
		indicatorStyle = styleStatusCompleted
	case "blocked":
		indicator = "⊘"
		indicatorStyle = styleStatusBlocked
	default:
		indicator = "○"
		indicatorStyle = styleStatusRemaining
	}

	// Truncate content to fit width (leave room for indicator and padding)
	maxContentWidth := s.width - 6 // 2 for border, 2 for indicator+space, 2 padding
	if maxContentWidth < 10 {
		maxContentWidth = 10
	}

	content := task.Content
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth-3] + "..."
	}

	// Build line with selection arrow or placeholder space
	styledIndicator := indicatorStyle.Render(indicator)
	prefix := " "
	if isSelected {
		prefix = "▸"
	}
	line := fmt.Sprintf("%s %s %s", prefix, styledIndicator, content)

	return line
}

// TaskAtPosition returns the task at the given screen coordinates, or nil if none.
// Uses coordinate-based hit detection against the tasks content area.
func (s *Sidebar) TaskAtPosition(x, y int) *session.Task {
	area := s.tasksContentArea
	// Check if the click is within the tasks content area
	if x < area.Min.X || x >= area.Max.X || y < area.Min.Y || y >= area.Max.Y {
		return nil
	}

	// Calculate which task line was clicked, accounting for viewport scroll
	lineIndex := (y - area.Min.Y) + s.tasksViewport.YOffset()

	tasks := s.getTasks()
	if lineIndex < 0 || lineIndex >= len(tasks) {
		return nil
	}

	return tasks[lineIndex]
}

// NoteAtPosition returns the note at the given screen coordinates, or nil if none.
// Uses coordinate-based hit detection against the notes content area.
func (s *Sidebar) NoteAtPosition(x, y int) *session.Note {
	area := s.notesContentArea
	// Check if the click is within the notes content area
	if x < area.Min.X || x >= area.Max.X || y < area.Min.Y || y >= area.Max.Y {
		return nil
	}

	// Calculate which note line was clicked, accounting for viewport scroll
	lineIndex := (y - area.Min.Y) + s.notesViewport.YOffset()

	// Notes display the last 10 notes
	if s.state == nil || len(s.state.Notes) == 0 {
		return nil
	}
	startIdx := 0
	if len(s.state.Notes) > 10 {
		startIdx = len(s.state.Notes) - 10
	}
	notes := s.state.Notes[startIdx:]

	if lineIndex < 0 || lineIndex >= len(notes) {
		return nil
	}

	return notes[lineIndex]
}

// SetActiveNote marks a note as active (highlighted) and rebuilds content.
func (s *Sidebar) SetActiveNote(id string) {
	s.activeNoteID = id
	s.updateContent()
}

// ClearActiveNote removes the active note highlight and rebuilds content.
func (s *Sidebar) ClearActiveNote() {
	s.activeNoteID = ""
	s.updateContent()
}

// SetActiveTask marks a task as active (highlighted) and rebuilds content.
func (s *Sidebar) SetActiveTask(id string) {
	s.activeTaskID = id
	s.updateContent()
}

// ClearActiveTask removes the active task highlight and rebuilds content.
func (s *Sidebar) ClearActiveTask() {
	s.activeTaskID = ""
	s.updateContent()
}

// SetFocus sets whether the sidebar has keyboard focus.
func (s *Sidebar) SetFocus(focused bool) {
	s.focused = focused
}

// IsFocused returns whether the sidebar has keyboard focus.
func (s *Sidebar) IsFocused() bool {
	return s.focused
}

// SetSize updates the sidebar dimensions and viewport sizes.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Calculate section heights (Tasks 60%, Notes 40%)
	tasksHeight := int(float64(height) * 0.6)
	if tasksHeight < 3 {
		tasksHeight = 3
	}
	notesHeight := height - tasksHeight
	if notesHeight < 2 {
		notesHeight = 2
		tasksHeight = height - notesHeight
	}

	// Account for borders and headers (2 chars each side, 2 lines for header/border)
	s.tasksViewport.SetWidth(width - 4)
	s.tasksViewport.SetHeight(tasksHeight - 4)
	s.notesViewport.SetWidth(width - 4)
	s.notesViewport.SetHeight(notesHeight - 4)
}

// SetState updates the sidebar with new session state.
func (s *Sidebar) SetState(state *session.State) {
	oldState := s.state
	s.state = state

	// Detect task status changes and mark for pulse
	if state != nil {
		statusChanged := false

		// Check for new or changed task statuses
		for id, task := range state.Tasks {
			oldStatus, existed := s.pulsedTaskIDs[id]

			// If task is new or status changed, mark for pulse
			// Only trigger pulse if we had a previous state (not initial load)
			if !existed || oldStatus != task.Status {
				s.pulsedTaskIDs[id] = task.Status
				if oldState != nil {
					statusChanged = true
				}
			}
		}

		// Set flag to start pulse on next Update
		if statusChanged {
			s.needsPulse = true
		}
	}

	s.updateContent()

	// Clamp cursor to valid range after state update
	tasks := s.getTasks()
	if len(tasks) == 0 {
		s.cursor = 0
	} else if s.cursor >= len(tasks) {
		s.cursor = len(tasks) - 1
	}
}

// rebuildIndex rebuilds the ID-based lookup indices for tasks and notes.
// This provides O(1) lookups by ID.
func (s *Sidebar) rebuildIndex() {
	if s.state == nil {
		s.taskIndex = make(map[string]int)
		s.noteIndex = make(map[string]int)
		return
	}

	// Rebuild task index: map task ID -> position in ordered task list
	// This provides O(1) lookup to find a task's position in the display order
	tasks := s.getTasks()
	s.taskIndex = make(map[string]int, len(tasks))
	for idx, task := range tasks {
		s.taskIndex[task.ID] = idx
	}

	// Rebuild note index: map note ID -> position in state.Notes slice
	s.noteIndex = make(map[string]int, len(s.state.Notes))
	for idx := range s.state.Notes {
		s.noteIndex[s.state.Notes[idx].ID] = idx
	}
}

// updateContent rebuilds viewport content from state.
func (s *Sidebar) updateContent() {
	if s.state == nil {
		return
	}

	// Rebuild indices for O(1) lookups
	s.rebuildIndex()

	// Update tasks viewport
	s.tasksViewport.SetContent(s.buildTasksContent())

	// Update notes viewport
	s.notesViewport.SetContent(s.buildNotesContent())
}

// buildNotesContent builds the content string for the notes viewport.
func (s *Sidebar) buildNotesContent() string {
	if len(s.state.Notes) == 0 {
		return styleDim.Render("  No notes")
	}

	var lines []string
	// Show recent notes (last 10)
	startIdx := 0
	if len(s.state.Notes) > 10 {
		startIdx = len(s.state.Notes) - 10
	}

	for _, note := range s.state.Notes[startIdx:] {
		isSelected := note.ID == s.activeNoteID
		line := s.renderNote(note, isSelected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderNote renders a single note line with type indicator and optional selection highlight.
func (s *Sidebar) renderNote(note *session.Note, isSelected bool) string {
	// Type indicator
	var indicator string
	var indicatorStyle lipgloss.Style

	switch note.Type {
	case "learning":
		indicator = "💡"
		indicatorStyle = styleStatusCompleted // Green-ish
	case "stuck":
		indicator = "🚫"
		indicatorStyle = styleStatusBlocked // Red
	case "tip":
		indicator = "💬"
		indicatorStyle = styleStatusInProgress // Yellow
	case "decision":
		indicator = "⚡"
		indicatorStyle = styleStatusRemaining // Blue
	default:
		indicator = "📝"
		indicatorStyle = styleDim
	}

	// Truncate content to fit width
	maxContentWidth := s.width - 8
	if maxContentWidth < 10 {
		maxContentWidth = 10
	}

	content := note.Content
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth-3] + "..."
	}

	// Build line with selection arrow or placeholder space
	styledIndicator := indicatorStyle.Render(indicator)
	prefix := " "
	if isSelected {
		prefix = "▸"
	}
	line := fmt.Sprintf("%s %s %s", prefix, styledIndicator, content)

	return line
}

// Render provides legacy string-based rendering for backward compatibility.
// This method will be removed in Phase 12 once App is refactored to use Screen/Draw pattern.
func (s *Sidebar) Render() string {
	// Guard against zero dimensions
	if s.width < 10 || s.height < 5 {
		return ""
	}

	// Calculate section heights (Tasks 60%, Notes 40%)
	tasksHeight := int(float64(s.height) * 0.6)
	if tasksHeight < 3 {
		tasksHeight = 3
	}
	notesHeight := s.height - tasksHeight
	if notesHeight < 2 {
		notesHeight = 2
		tasksHeight = s.height - notesHeight
	}

	// Render tasks section
	tasksHeader := styleSidebarHeader.Width(s.width - 2).Render("Tasks")
	tasksContent := s.tasksViewport.View()
	tasksSection := lipgloss.JoinVertical(lipgloss.Left, tasksHeader, tasksContent)
	tasksBox := styleSidebarBorder.Width(s.width).Height(tasksHeight).Render(tasksSection)

	// Render notes section
	notesHeader := styleSidebarHeader.Width(s.width - 2).Render("Notes")
	notesContent := s.notesViewport.View()
	notesSection := lipgloss.JoinVertical(lipgloss.Left, notesHeader, notesContent)
	notesBox := styleSidebarBorder.Width(s.width).Height(notesHeight).Render(notesSection)

	// Join sections vertically
	return lipgloss.JoinVertical(lipgloss.Left, tasksBox, notesBox)
}

// Legacy methods for backward compatibility
func (s *Sidebar) SetFocused(focused bool)                  { s.SetFocus(focused) }
func (s *Sidebar) UpdateSize(width, height int) tea.Cmd     { s.SetSize(width, height); return nil }
func (s *Sidebar) UpdateState(state *session.State) tea.Cmd { s.SetState(state); return nil }

// GetTaskByID returns a task by ID using O(1) lookup via taskIndex.
// Returns nil if task not found.
func (s *Sidebar) GetTaskByID(id string) *session.Task {
	if s.state == nil {
		return nil
	}
	// Use taskIndex to find position in ordered task list
	idx, ok := s.taskIndex[id]
	if !ok {
		return nil
	}
	tasks := s.getTasks()
	if idx < 0 || idx >= len(tasks) {
		return nil
	}
	return tasks[idx]
}

// GetNoteByID returns a note by ID using O(1) lookup.
// Returns nil if note not found.
func (s *Sidebar) GetNoteByID(id string) *session.Note {
	if s.state == nil {
		return nil
	}
	idx, ok := s.noteIndex[id]
	if !ok || idx < 0 || idx >= len(s.state.Notes) {
		return nil
	}
	return s.state.Notes[idx]
}

// Compile-time interface check
var _ FocusableComponent = (*Sidebar)(nil)

// Sidebar styles (used by legacy Render method, will be removed in Phase 12)
var (
	styleSidebarBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, true, true, false). // No left border
				BorderForeground(colorMuted)

	styleSidebarHeader = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(colorMuted).
				PaddingLeft(1)
)
