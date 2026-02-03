package specwizard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mark3labs/iteratr/internal/tui/theme"
	"github.com/mark3labs/iteratr/internal/tui/wizard"
)

// CompletionStep shows the completion screen with Build/Exit buttons.
type CompletionStep struct {
	specPath      string
	width         int
	height        int
	buttonBar     *wizard.ButtonBar
	buttonFocused bool
}

// NewCompletionStep creates a new completion step.
func NewCompletionStep(specPath string) *CompletionStep {
	// Create button bar with Start Build and Exit buttons
	buttons := []wizard.Button{
		{Label: "Start Build", State: wizard.ButtonNormal},
		{Label: "Exit", State: wizard.ButtonNormal},
	}
	buttonBar := wizard.NewButtonBar(buttons)

	return &CompletionStep{
		specPath:      specPath,
		buttonBar:     buttonBar,
		buttonFocused: true, // Auto-focus buttons on entry
	}
}

// Init initializes the completion step.
func (s *CompletionStep) Init() tea.Cmd {
	// Auto-focus first button
	if s.buttonBar != nil {
		s.buttonBar.FocusFirst()
	}
	return nil
}

// Update handles messages for the completion step.
func (s *CompletionStep) Update(msg tea.Msg) tea.Cmd {
	// Handle button-focused keyboard input
	if s.buttonFocused && s.buttonBar != nil {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "tab", "right":
				if !s.buttonBar.FocusNext() {
					// Wrapped around - go back to first button
					s.buttonBar.FocusFirst()
				}
				return nil
			case "shift+tab", "left":
				if !s.buttonBar.FocusPrev() {
					// Wrapped around - go to last button
					s.buttonBar.FocusLast()
				}
				return nil
			case "enter", " ":
				return s.activateButton(s.buttonBar.FocusedButton())
			}
		}
	}

	return nil
}

// View renders the completion step.
func (s *CompletionStep) View() string {
	currentTheme := theme.Current()
	var b strings.Builder

	// Success icon
	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(currentTheme.Success)).
		Bold(true).
		MarginBottom(1)
	b.WriteString(iconStyle.Render("✓ Spec Created Successfully!"))
	b.WriteString("\n\n")

	// Spec path
	pathLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(currentTheme.FgMuted))
	pathValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(currentTheme.Primary)).
		Bold(true)

	b.WriteString(pathLabelStyle.Render("Saved to: "))
	b.WriteString(pathValueStyle.Render(s.specPath))
	b.WriteString("\n\n")

	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(currentTheme.FgBase)).
		MarginBottom(1)
	b.WriteString(instructionStyle.Render("What would you like to do next?"))
	b.WriteString("\n\n")

	// Button bar
	if s.buttonBar != nil {
		b.WriteString(s.buttonBar.Render())
		b.WriteString("\n")
	}

	// Hint bar
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(currentTheme.FgMuted))
	b.WriteString(hintStyle.Render("tab/arrow keys to navigate • enter to select"))

	return b.String()
}

// SetSize updates the size of the completion step.
func (s *CompletionStep) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Update button bar width
	if s.buttonBar != nil {
		s.buttonBar.SetWidth(width)
	}
}

// activateButton handles button activation.
func (s *CompletionStep) activateButton(btnID wizard.ButtonID) tea.Cmd {
	switch btnID {
	case wizard.ButtonBack:
		// Start Build button (first button maps to ButtonBack in 2-button layout)
		return func() tea.Msg {
			return StartBuildMsg{}
		}
	case wizard.ButtonNext:
		// Exit button (second button maps to ButtonNext)
		return tea.Quit
	}
	return nil
}

// StartBuildMsg is sent when the user clicks "Start Build".
type StartBuildMsg struct{}
