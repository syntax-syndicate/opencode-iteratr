package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/mark3labs/iteratr/internal/session"
)

// Compile-time interface checks
var _ Component = (*Dashboard)(nil)

// FocusArea represents which part of the dashboard has keyboard focus.
type FocusArea int

const (
	FocusMain FocusArea = iota
	FocusSidebar
)

// SidebarWidth is the fixed width for the task sidebar.
const SidebarWidth = 45

// Dashboard displays session overview, progress, and current task.
type Dashboard struct {
	sessionName string
	iteration   int
	state       *session.State
	width       int
	height      int
	agentOutput *AgentOutput // Reference to agent output for rendering
	sidebar     *TaskSidebar // Task sidebar on the right
	focus       FocusArea    // Which area has keyboard focus
}

// NewDashboard creates a new Dashboard component.
func NewDashboard(agentOutput *AgentOutput) *Dashboard {
	return &Dashboard{
		agentOutput: agentOutput,
		sidebar:     NewTaskSidebar(),
		focus:       FocusMain,
	}
}

// Update handles messages for the dashboard.
func (d *Dashboard) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Tab switches focus between main and sidebar
		if msg.String() == "tab" {
			if d.focus == FocusMain {
				d.focus = FocusSidebar
				d.sidebar.SetFocused(true)
			} else {
				d.focus = FocusMain
				d.sidebar.SetFocused(false)
			}
			return nil
		}

		// Forward keys based on focus
		if d.focus == FocusSidebar {
			return d.sidebar.Update(msg)
		}
	}

	// Forward scroll events to agent output viewport when main is focused
	if d.agentOutput != nil && d.focus == FocusMain {
		return d.agentOutput.Update(msg)
	}
	return nil
}

// Draw renders the dashboard to a screen buffer using the Screen/Draw pattern.
func (d *Dashboard) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// Draw panel border with title
	focusTitle := "Agent Output"
	if d.focus == FocusMain {
		focusTitle = "Agent Output"
	}
	inner := DrawPanel(scr, area, focusTitle, d.focus == FocusMain)

	// Delegate to AgentOutput.Draw for content rendering
	if d.agentOutput != nil {
		return d.agentOutput.Draw(scr, inner)
	}

	return nil
}

// Render returns the dashboard view as a string.
func (d *Dashboard) Render() string {
	// Calculate widths
	sidebarWidth := SidebarWidth
	mainWidth := d.width - sidebarWidth
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Render main content area (left side)
	mainContent := d.renderMainContent(mainWidth)

	// Render sidebar (right side)
	sidebarContent := d.sidebar.Render()

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, mainContent, sidebarContent)
}

// renderMainContent renders the main content area (session info + agent output).
func (d *Dashboard) renderMainContent(width int) string {
	// Build header sections (fixed height)
	var headerSections []string

	// Section 1: Session Info
	sessionInfo := d.renderSessionInfo()
	headerSections = append(headerSections, sessionInfo)

	// Section 2: Progress Indicator
	if d.state != nil {
		progressInfo := d.renderProgressIndicator()
		headerSections = append(headerSections, "") // blank line
		headerSections = append(headerSections, progressInfo)
	}

	// Render header
	header := lipgloss.JoinVertical(lipgloss.Left, headerSections...)

	// Section 3: Agent Output (takes remaining space)
	var agentSection string
	if d.agentOutput != nil {
		focusIndicator := ""
		if d.focus == FocusMain {
			focusIndicator = " " + styleStatusInProgress.Render("●")
		}
		agentLabel := styleStatLabel.Render("Agent Output:") + focusIndicator
		agentContent := d.agentOutput.Render()
		agentSection = lipgloss.JoinVertical(lipgloss.Left, "", agentLabel, "", agentContent)
	}

	// Join header and agent sections
	content := lipgloss.JoinVertical(lipgloss.Left, header, agentSection)

	// Apply width constraint
	return lipgloss.NewStyle().Width(width).Render(content)
}

// renderSessionInfo renders the session name and iteration number.
func (d *Dashboard) renderSessionInfo() string {
	var parts []string

	// Session name
	sessionLabel := styleStatLabel.Render("Session:")
	sessionValue := styleStatValue.Render(d.sessionName)
	parts = append(parts, sessionLabel+" "+sessionValue)

	// Iteration number
	iterationLabel := styleStatLabel.Render("Iteration:")
	iterationValue := styleStatValue.Render(fmt.Sprintf("#%d", d.iteration))
	parts = append(parts, iterationLabel+" "+iterationValue)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// UpdateSize updates the dashboard dimensions.
func (d *Dashboard) UpdateSize(width, height int) tea.Cmd {
	d.width = width
	d.height = height

	// Calculate widths
	sidebarWidth := SidebarWidth
	mainWidth := width - sidebarWidth
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Update sidebar size
	d.sidebar.UpdateSize(sidebarWidth, height)

	// Update agent output viewport size
	// Reserve space for: session info (2) + progress (2) + current task (3) + agent label (2) + padding (3)
	if d.agentOutput != nil {
		agentHeight := height - 12
		if agentHeight < 5 {
			agentHeight = 5
		}
		d.agentOutput.UpdateSize(mainWidth-2, agentHeight) // -2 for padding
	}
	return nil
}

// SetIteration sets the current iteration number.
func (d *Dashboard) SetIteration(n int) tea.Cmd {
	d.iteration = n
	return nil
}

// UpdateState updates the dashboard with new session state.
func (d *Dashboard) UpdateState(state *session.State) tea.Cmd {
	d.state = state
	// Update session name from state
	if state != nil {
		d.sessionName = state.Session
	}
	// Update sidebar state
	d.sidebar.UpdateState(state)
	return nil
}

// renderProgressIndicator renders a progress bar showing task completion.
func (d *Dashboard) renderProgressIndicator() string {
	// Count tasks by status
	stats := d.getTaskStats()

	// Build progress bar
	const barWidth = 40
	var completedWidth int
	if stats.Total > 0 {
		completedWidth = (stats.Completed * barWidth) / stats.Total
	}

	// Create the bar with filled and empty portions
	filled := ""
	empty := ""
	for i := 0; i < completedWidth; i++ {
		filled += "█"
	}
	for i := completedWidth; i < barWidth; i++ {
		empty += "░"
	}

	// Format the progress text
	progressText := fmt.Sprintf("%d/%d tasks", stats.Completed, stats.Total)

	// Combine bar and text
	bar := styleProgressFill.Render(filled) + styleDim.Render(empty)
	label := styleStatLabel.Render("Progress:")
	return fmt.Sprintf("%s [%s] %s", label, bar, styleStatValue.Render(progressText))
}

// progressStats holds task statistics.
type progressStats struct {
	Total      int
	Remaining  int
	InProgress int
	Completed  int
	Blocked    int
}

// getTaskStats computes task statistics.
func (d *Dashboard) getTaskStats() progressStats {
	var stats progressStats
	if d.state == nil {
		return stats
	}
	for _, task := range d.state.Tasks {
		stats.Total++
		switch task.Status {
		case "remaining":
			stats.Remaining++
		case "in_progress":
			stats.InProgress++
		case "completed":
			stats.Completed++
		case "blocked":
			stats.Blocked++
		}
	}
	return stats
}
