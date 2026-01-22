package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
)

// FooterAction represents a clickable action in the footer.
type FooterAction string

const (
	FooterActionDashboard FooterAction = "dashboard"
	FooterActionLogs      FooterAction = "logs"
	FooterActionNotes     FooterAction = "notes"
	FooterActionInbox     FooterAction = "inbox"
	FooterActionSidebar   FooterAction = "sidebar"
	FooterActionHelp      FooterAction = "help"
	FooterActionQuit      FooterAction = "quit"
)

// footerButton tracks the hit region for a clickable footer button.
type footerButton struct {
	action FooterAction
	startX int // inclusive
	endX   int // exclusive
}

// Footer renders the bottom footer bar with navigation hints.
type Footer struct {
	width      int
	activeView ViewType
	layoutMode LayoutMode
	area       uv.Rectangle   // Screen area where footer is drawn
	buttons    []footerButton // Clickable hit regions
}

// NewFooter creates a new Footer component.
func NewFooter() *Footer {
	return &Footer{
		layoutMode: LayoutDesktop,
	}
}

// Draw renders the footer to the screen at the given area.
// Returns nil cursor since footer is non-interactive.
func (f *Footer) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if area.Dy() < 1 {
		return nil
	}

	// Store area for mouse hit detection
	f.area = area

	// Build footer content based on available width
	content := f.buildFooterContent(area.Dx())

	// Render to screen using DrawStyled helper
	DrawStyled(scr, area, styleFooter, content)

	return nil
}

// buildFooterContent creates the footer text with navigation hints.
func (f *Footer) buildFooterContent(availableWidth int) string {
	type buttonPart struct {
		rendered string
		action   FooterAction
	}

	// View navigation shortcuts
	views := []struct {
		key    string
		name   string
		view   ViewType
		action FooterAction
	}{
		{"1", "Dashboard", ViewDashboard, FooterActionDashboard},
		{"2", "Logs", ViewLogs, FooterActionLogs},
		{"3", "Notes", ViewNotes, FooterActionNotes},
		{"4", "Inbox", ViewInbox, FooterActionInbox},
	}

	var leftButtons []buttonPart
	for _, v := range views {
		key := styleFooterKey.Render(fmt.Sprintf("[%s]", v.key))
		var label string
		if v.view == f.activeView {
			label = styleFooterActive.Render(v.name)
		} else {
			label = styleFooterLabel.Render(v.name)
		}
		leftButtons = append(leftButtons, buttonPart{
			rendered: key + " " + label,
			action:   v.action,
		})
	}

	// In compact mode, add sidebar toggle hint
	if f.layoutMode == LayoutCompact {
		leftButtons = append(leftButtons, buttonPart{
			rendered: styleFooterKey.Render("[s]") + styleFooterLabel.Render("Sidebar"),
			action:   FooterActionSidebar,
		})
	}

	rightButtons := []buttonPart{
		{rendered: styleFooterKey.Render("[?]") + styleFooterLabel.Render("Help"), action: FooterActionHelp},
		{rendered: styleFooterKey.Render("[q]") + styleFooterLabel.Render("Quit"), action: FooterActionQuit},
	}

	// Build left and right strings
	var leftParts []string
	for _, b := range leftButtons {
		leftParts = append(leftParts, b.rendered)
	}
	left := strings.Join(leftParts, "  ")

	var rightParts []string
	for _, b := range rightButtons {
		rightParts = append(rightParts, b.rendered)
	}
	right := strings.Join(rightParts, "  ")

	// Calculate spacing to fill width
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := availableWidth - leftWidth - rightWidth - 2 // -2 for side padding
	if padding < 2 {
		padding = 2
	}

	// Combine with spacing
	content := left + strings.Repeat(" ", padding) + right

	// If content is too wide, use condensed version (no button tracking for condensed)
	if lipgloss.Width(content) > availableWidth {
		f.buttons = nil
		return f.buildCondensedContent(availableWidth)
	}

	// Track button hit regions (accounting for 1 char left padding from styleFooter)
	f.buttons = nil
	xOffset := f.area.Min.X + 1 // 1 for styleFooter left padding

	for i, b := range leftButtons {
		w := lipgloss.Width(b.rendered)
		f.buttons = append(f.buttons, footerButton{
			action: b.action,
			startX: xOffset,
			endX:   xOffset + w,
		})
		xOffset += w
		if i < len(leftButtons)-1 {
			xOffset += 2 // separator "  "
		}
	}

	// Right side buttons start after left + padding
	xOffset = f.area.Min.X + 1 + leftWidth + padding
	for i, b := range rightButtons {
		w := lipgloss.Width(b.rendered)
		f.buttons = append(f.buttons, footerButton{
			action: b.action,
			startX: xOffset,
			endX:   xOffset + w,
		})
		xOffset += w
		if i < len(rightButtons)-1 {
			xOffset += 2 // separator "  "
		}
	}

	return content
}

// buildCondensedContent creates a shorter version for narrow terminals.
func (f *Footer) buildCondensedContent(availableWidth int) string {
	// Minimal version: [1-4]Views [?]Help [q]Quit
	views := styleFooterKey.Render("[1-4]") + styleFooterLabel.Render("Views")
	help := styleFooterKey.Render("[?]") + styleFooterLabel.Render("Help")
	quit := styleFooterKey.Render("[q]") + styleFooterLabel.Render("Quit")

	parts := []string{views, help, quit}
	content := strings.Join(parts, " ")

	// If still too wide, use ultra-minimal version
	if lipgloss.Width(content) > availableWidth {
		content = styleFooterKey.Render("[1-4]") + " " +
			styleFooterKey.Render("[?]") + " " +
			styleFooterKey.Render("[q]")
	}

	return content
}

// ActionAtPosition returns the footer action at the given screen coordinates, or empty string if none.
func (f *Footer) ActionAtPosition(x, y int) FooterAction {
	// Check Y is within footer area
	if y < f.area.Min.Y || y >= f.area.Max.Y {
		return ""
	}

	for _, b := range f.buttons {
		if x >= b.startX && x < b.endX {
			return b.action
		}
	}
	return ""
}

// SetSize updates the footer width.
func (f *Footer) SetSize(width, height int) {
	f.width = width
}

// SetActiveView updates which view is currently active.
func (f *Footer) SetActiveView(view ViewType) {
	f.activeView = view
}

// SetLayoutMode updates the layout mode (desktop/compact).
func (f *Footer) SetLayoutMode(mode LayoutMode) {
	f.layoutMode = mode
}

// Update handles messages. Footer is mostly static.
func (f *Footer) Update(msg tea.Msg) tea.Cmd {
	return nil
}

// Compile-time interface check
var _ Component = (*Footer)(nil)
