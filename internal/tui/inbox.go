package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/mark3labs/iteratr/internal/session"
)

// InboxPanel displays unread messages and provides an input field for sending.
type InboxPanel struct {
	state  *session.State
	width  int
	height int
}

// NewInboxPanel creates a new InboxPanel component.
func NewInboxPanel() *InboxPanel {
	return &InboxPanel{}
}

// Update handles messages for the inbox panel.
func (i *InboxPanel) Update(msg tea.Msg) tea.Cmd {
	// TODO: Implement inbox panel updates (input handling)
	return nil
}

// Render returns the inbox panel view as a string.
func (i *InboxPanel) Render() string {
	if i.state == nil {
		return styleEmptyState.Render("No session state loaded")
	}

	var content strings.Builder

	// Title
	content.WriteString(stylePanelTitle.Render("Inbox"))
	content.WriteString("\n\n")

	// Filter unread messages
	var unreadMessages []*session.Message
	for _, msg := range i.state.Inbox {
		if !msg.Read {
			unreadMessages = append(unreadMessages, msg)
		}
	}

	// Display unread messages
	if len(unreadMessages) == 0 {
		content.WriteString(styleEmptyState.Render("No unread messages"))
	} else {
		// Show count
		content.WriteString(styleBadgeInfo.Render(fmt.Sprintf("%d unread", len(unreadMessages))))
		content.WriteString("\n\n")

		// Render each message
		for _, msg := range unreadMessages {
			content.WriteString(i.renderMessage(msg))
			content.WriteString("\n")
		}
	}

	// Render in panel style
	return stylePanel.Width(i.width - 4).Height(i.height - 4).Render(content.String())
}

// renderMessage renders a single inbox message.
func (i *InboxPanel) renderMessage(msg *session.Message) string {
	// Message ID (first 8 chars)
	idPrefix := msg.ID
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8]
	}

	// Format: [id] content
	var parts []string
	parts = append(parts, styleMessageUnread.Render(fmt.Sprintf("[%s]", idPrefix)))
	parts = append(parts, styleMessageUnread.Render(msg.Content))

	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// UpdateSize updates the inbox panel dimensions.
func (i *InboxPanel) UpdateSize(width, height int) tea.Cmd {
	i.width = width
	i.height = height
	return nil
}

// UpdateState updates the inbox panel with new session state.
func (i *InboxPanel) UpdateState(state *session.State) tea.Cmd {
	i.state = state
	return nil
}
