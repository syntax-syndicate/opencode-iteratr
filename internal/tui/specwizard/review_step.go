package specwizard

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/editor"
	"github.com/mark3labs/iteratr/internal/config"
)

// ReviewStep handles the spec review and editing step with markdown rendering.
type ReviewStep struct {
	viewport viewport.Model // Scrollable viewport for spec display
	content  string         // Raw spec markdown content
	cfg      *config.Config
	width    int    // Available width
	height   int    // Available height
	tmpFile  string // Path to temp file for editing
	edited   bool   // True if user edited via external editor
}

// NewReviewStep creates a new review step.
func NewReviewStep(content string, cfg *config.Config) *ReviewStep {
	// Create viewport with initial dimensions
	vp := viewport.New(
		viewport.WithWidth(60),
		viewport.WithHeight(10),
	)

	// Enable mouse wheel scrolling
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	// Set rendered markdown content
	vp.SetContent(renderMarkdown(content, 60))

	return &ReviewStep{
		viewport: vp,
		content:  content,
		cfg:      cfg,
		width:    60,
		height:   20,
	}
}

// renderMarkdown renders markdown content with syntax highlighting using glamour.
// Falls back to plain text if rendering fails.
func renderMarkdown(content string, width int) string {
	// Cap width to 120 for readability
	if width > 120 {
		width = 120
	}

	// Create glamour renderer with dark theme and word wrap
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to plain text
		return content
	}

	// Render the markdown
	rendered, err := r.Render(content)
	if err != nil {
		// Fallback to plain text
		return content
	}

	// Remove trailing newline that glamour adds
	return strings.TrimSuffix(rendered, "\n")
}

// Init initializes the review step.
func (s *ReviewStep) Init() tea.Cmd {
	return nil
}

// SetSize updates the dimensions for the review step.
func (s *ReviewStep) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Update viewport dimensions (full width)
	s.viewport.SetWidth(width)

	// Reserve space for hint bar (1 line)
	viewportHeight := height - 1
	if viewportHeight < 5 {
		viewportHeight = 5
	}
	s.viewport.SetHeight(viewportHeight)

	// Re-render markdown with new width
	s.viewport.SetContent(renderMarkdown(s.content, width))
}

// Update handles messages for the review step.
func (s *ReviewStep) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "e":
			// Open external editor if $EDITOR is set
			if os.Getenv("EDITOR") != "" {
				return s.openEditor()
			}
		}
	case SpecEditedMsg:
		// Editor returned with new content
		s.content = msg.Content
		s.edited = true
		s.viewport.SetContent(renderMarkdown(s.content, s.width))
		s.viewport.GotoTop()
		// Clean up temp file
		if s.tmpFile != "" {
			_ = os.Remove(s.tmpFile)
			s.tmpFile = ""
		}
		return nil
	}

	// Forward viewport messages (scrolling, etc.)
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return cmd
}

// openEditor launches the user's $EDITOR with the spec content.
func (s *ReviewStep) openEditor() tea.Cmd {
	// Create temp file with spec content
	tmpfile, err := os.CreateTemp("", "iteratr_spec_*.md")
	if err != nil {
		return nil // Silently fail - editor not available
	}

	// Write current content to temp file
	if _, err := tmpfile.WriteString(s.content); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return nil
	}
	_ = tmpfile.Close()

	// Store temp file path for cleanup
	s.tmpFile = tmpfile.Name()

	// Create editor command
	cmd, err := editor.Command("iteratr", tmpfile.Name())
	if err != nil {
		_ = os.Remove(tmpfile.Name())
		return nil
	}

	// Execute editor and read result
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return nil
		}

		// Read modified content
		content, err := os.ReadFile(tmpfile.Name())
		if err != nil {
			return nil
		}

		return SpecEditedMsg{
			Content: string(content),
		}
	})
}

// View renders the review step.
func (s *ReviewStep) View() string {
	var b strings.Builder

	// Render viewport with markdown content
	b.WriteString(s.viewport.View())
	b.WriteString("\n")

	// Hint bar - show edit option if $EDITOR is set
	var hintBar string
	if os.Getenv("EDITOR") != "" {
		hintBar = renderHintBar(
			"↑↓", "scroll",
			"e", "edit",
			"tab", "buttons",
			"esc", "back",
		)
	} else {
		hintBar = renderHintBar(
			"↑↓", "scroll",
			"tab", "buttons",
			"esc", "back",
		)
	}
	b.WriteString(hintBar)

	return b.String()
}

// Content returns the current spec content (possibly edited).
func (s *ReviewStep) Content() string {
	return s.content
}

// WasEdited returns true if the user edited the spec via external editor.
func (s *ReviewStep) WasEdited() bool {
	return s.edited
}

// SpecEditedMsg is sent when the external editor returns with new content.
type SpecEditedMsg struct {
	Content string
}
