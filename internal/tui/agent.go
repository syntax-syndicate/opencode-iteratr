package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// AgentOutput displays streaming agent output with auto-scroll.
type AgentOutput struct {
	viewport          viewport.Model
	messages          []MessageItem
	toolIndex         map[string]int // toolCallId → message index
	width             int
	height            int
	autoScroll        bool // Whether to auto-scroll to bottom on new content
	ready             bool // Whether viewport is initialized
	spinner           *GradientSpinner
	isStreaming       bool
	focusedIndex      int          // Index of the message that has keyboard focus (-1 = no focus)
	viewportArea      uv.Rectangle // Screen area where viewport is drawn (for mouse hit detection)
	messageLineStarts []int        // Start line index in content for each message
}

// Compile-time interface checks
var _ Drawable = (*AgentOutput)(nil)
var _ Updateable = (*AgentOutput)(nil)
var _ Component = (*AgentOutput)(nil)

// NewAgentOutput creates a new AgentOutput component.
func NewAgentOutput() *AgentOutput {
	return &AgentOutput{
		messages:     make([]MessageItem, 0),
		toolIndex:    make(map[string]int),
		autoScroll:   true,
		focusedIndex: -1, // No message focused initially
	}
}

// Init initializes the agent output component.
func (a *AgentOutput) Init() tea.Cmd {
	return nil
}

// Update handles messages for the agent output.
func (a *AgentOutput) Update(msg tea.Msg) tea.Cmd {
	if !a.ready {
		return nil
	}

	// Handle gradient spinner animation
	if a.spinner != nil {
		if spinnerCmd := a.spinner.Update(msg); spinnerCmd != nil {
			// Refresh content to show updated spinner frame
			a.refreshContent()
			return spinnerCmd
		}
	}

	// Handle keyboard input for expand/collapse on focused message
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "up":
			// Move focus to previous expandable message
			a.moveFocusPrevious()
			return nil
		case "down":
			// Move focus to next expandable message
			a.moveFocusNext()
			return nil
		case "space", "enter":
			// Toggle expansion on focused message if it's expandable
			if a.focusedIndex >= 0 && a.focusedIndex < len(a.messages) {
				focusedMsg := a.messages[a.focusedIndex]
				if expandable, ok := focusedMsg.(Expandable); ok {
					expandable.ToggleExpanded()
					a.refreshContent()
					return nil
				}
			}
		}
	}

	var cmd tea.Cmd
	a.viewport, cmd = a.viewport.Update(msg)

	// Check if user manually scrolled - disable auto-scroll
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseMsg:
		if !a.viewport.AtBottom() {
			a.autoScroll = false
		} else {
			a.autoScroll = true
		}
	}

	return cmd
}

// Render returns the agent output view as a string.
func (a *AgentOutput) Render() string {
	if !a.ready {
		return styleDim.Render("Waiting for agent output...")
	}
	return a.viewport.View()
}

// Draw renders the agent output to a screen buffer.
func (a *AgentOutput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if !a.ready {
		// Show waiting message
		waitMsg := styleDim.Render("Waiting for agent output...")
		uv.NewStyledString(waitMsg).Draw(scr, area)
		return nil
	}

	// Render viewport content with 1-char left margin
	content := a.viewport.View()
	contentArea := uv.Rect(area.Min.X+1, area.Min.Y, area.Dx()-1, area.Dy())
	a.viewportArea = contentArea
	uv.NewStyledString(content).Draw(scr, contentArea)

	// Draw scroll indicator if there's overflow
	if a.viewport.TotalLineCount() > a.viewport.Height() {
		pct := a.viewport.ScrollPercent()
		indicator := fmt.Sprintf(" %d%% ", int(pct*100))

		// Position indicator at bottom-right of area
		indicatorArea := uv.Rect(
			area.Max.X-len(indicator),
			area.Max.Y-1,
			len(indicator),
			1,
		)

		styledIndicator := styleScrollIndicator.Render(indicator)
		uv.NewStyledString(styledIndicator).Draw(scr, indicatorArea)
	}

	return nil
}

// UpdateSize updates the agent output dimensions.
func (a *AgentOutput) UpdateSize(width, height int) tea.Cmd {
	a.width = width
	a.height = height

	if !a.ready {
		a.viewport = viewport.New(
			viewport.WithWidth(width),
			viewport.WithHeight(height),
		)
		a.viewport.MouseWheelEnabled = true
		a.viewport.MouseWheelDelta = 3
		a.ready = true
	} else {
		a.viewport.SetWidth(width)
		a.viewport.SetHeight(height)
	}

	a.refreshContent()
	return nil
}

// AppendText adds a text message to the output.
func (a *AgentOutput) AppendText(content string) tea.Cmd {
	// Start spinner on first streaming content
	var spinnerCmd tea.Cmd
	if !a.isStreaming && content == "" {
		a.isStreaming = true
		// Create spinner with label "Generating..."
		spinner := NewGradientSpinner("#cba6f7", "#89b4fa", "Generating...")
		a.spinner = &spinner
		spinnerCmd = a.spinner.Tick()
	}

	// Stop spinner when actual content arrives
	if a.isStreaming && content != "" {
		a.isStreaming = false
		a.spinner = nil
	}

	// If last message is a TextMessageItem, append to it
	if len(a.messages) > 0 {
		if textMsg, ok := a.messages[len(a.messages)-1].(*TextMessageItem); ok {
			textMsg.content += content
			// Invalidate cache
			textMsg.cachedWidth = 0
			a.refreshContent()
			return spinnerCmd
		}
	}

	// Create new TextMessageItem
	newMsg := &TextMessageItem{
		id:      fmt.Sprintf("text-%d", len(a.messages)),
		content: content,
	}
	a.messages = append(a.messages, newMsg)
	a.refreshContent()
	return spinnerCmd
}

// AppendToolCall handles tool lifecycle events.
// If toolCallId not in toolIndex: append new message, store index.
// If toolCallId exists: update message in-place (status, input, output).
func (a *AgentOutput) AppendToolCall(msg AgentToolCallMsg) tea.Cmd {
	idx, exists := a.toolIndex[msg.ToolCallID]
	if !exists {
		// New tool call - create ToolMessageItem
		// Map status strings to ToolStatus enum
		status := mapToolStatus(msg.Status)

		newMsg := &ToolMessageItem{
			id:       msg.ToolCallID,
			toolName: msg.Title,
			status:   status,
			input:    msg.Input,
			output:   msg.Output,
			maxLines: 10,
		}
		a.messages = append(a.messages, newMsg)
		a.toolIndex[msg.ToolCallID] = len(a.messages) - 1
	} else {
		// Update existing tool call in-place
		if toolMsg, ok := a.messages[idx].(*ToolMessageItem); ok {
			toolMsg.status = mapToolStatus(msg.Status)
			if len(msg.Input) > 0 {
				toolMsg.input = msg.Input
			}
			if msg.Output != "" {
				toolMsg.output = msg.Output
			}
			// Invalidate cache
			toolMsg.cachedWidth = 0
		}
	}
	a.refreshContent()
	return nil
}

// mapToolStatus converts status strings to ToolStatus enum.
func mapToolStatus(status string) ToolStatus {
	switch status {
	case "pending":
		return ToolStatusPending
	case "in_progress":
		return ToolStatusRunning
	case "completed":
		return ToolStatusSuccess
	case "error":
		return ToolStatusError
	case "canceled", "cancelled":
		return ToolStatusCanceled
	default:
		return ToolStatusPending
	}
}

// AddIterationDivider adds a horizontal divider for a new iteration.
func (a *AgentOutput) AddIterationDivider(iteration int) tea.Cmd {
	newMsg := &DividerMessageItem{
		id:        fmt.Sprintf("divider-%d", iteration),
		iteration: iteration,
	}
	a.messages = append(a.messages, newMsg)
	a.refreshContent()
	return nil
}

// HandleClick processes a mouse click at screen coordinates (x, y).
// Returns true if the click toggled an expandable message.
func (a *AgentOutput) HandleClick(x, y int) bool {
	if !a.ready || len(a.messageLineStarts) == 0 {
		return false
	}

	// Check if click is within the viewport area
	if x < a.viewportArea.Min.X || x >= a.viewportArea.Max.X ||
		y < a.viewportArea.Min.Y || y >= a.viewportArea.Max.Y {
		return false
	}

	// Translate screen Y to content line (accounting for scroll offset)
	contentLine := (y - a.viewportArea.Min.Y) + a.viewport.YOffset()

	// Find which message this line belongs to
	msgIdx := -1
	for i := len(a.messageLineStarts) - 1; i >= 0; i-- {
		if contentLine >= a.messageLineStarts[i] {
			msgIdx = i
			break
		}
	}

	if msgIdx < 0 || msgIdx >= len(a.messages) {
		return false
	}

	// Toggle if expandable
	if expandable, ok := a.messages[msgIdx].(Expandable); ok {
		expandable.ToggleExpanded()
		a.refreshContent()
		return true
	}

	return false
}

// refreshContent rebuilds the viewport content from messages.
func (a *AgentOutput) refreshContent() {
	if !a.ready {
		return
	}

	var rendered strings.Builder
	// Account for border, padding, and 1-char left margin
	contentWidth := a.width - 5
	currentLine := 0
	a.messageLineStarts = make([]int, 0, len(a.messages))

	// If streaming and no content yet, prepend spinner view
	if a.isStreaming && a.spinner != nil && len(a.messages) == 0 {
		rendered.WriteString(a.spinner.View())
		rendered.WriteString("\n")
		currentLine++
	}

	for i, msg := range a.messages {
		a.messageLineStarts = append(a.messageLineStarts, currentLine)
		block := msg.Render(contentWidth)
		rendered.WriteString(block)
		rendered.WriteString("\n")
		currentLine += strings.Count(block, "\n") + 1

		// Add vertical spacing between message blocks
		// Skip extra spacing after dividers (they have their own margins)
		if i < len(a.messages)-1 {
			if _, isDivider := msg.(*DividerMessageItem); !isDivider {
				rendered.WriteString("\n")
				currentLine++
			}
		}
	}

	a.viewport.SetContent(rendered.String())

	if a.autoScroll {
		a.viewport.GotoBottom()
	}
}

// wrapText wraps text to the given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Wrap long lines
		for len(line) > width {
			// Find last space before width
			breakPoint := width
			for j := width; j > 0; j-- {
				if line[j] == ' ' {
					breakPoint = j
					break
				}
			}
			result.WriteString(line[:breakPoint])
			result.WriteString("\n")
			line = strings.TrimLeft(line[breakPoint:], " ")
		}
		result.WriteString(line)
	}

	return result.String()
}

// Clear resets the agent output content.
func (a *AgentOutput) Clear() tea.Cmd {
	a.messages = make([]MessageItem, 0)
	a.toolIndex = make(map[string]int)
	if a.ready {
		a.viewport.SetContent("")
		a.viewport.GotoTop()
	}
	a.autoScroll = true
	return nil
}

// AppendThinking adds thinking/reasoning content to the output.
// If last message is ThinkingMessageItem, appends to it; otherwise creates new one.
func (a *AgentOutput) AppendThinking(content string) tea.Cmd {
	// Start spinner on first streaming content
	var spinnerCmd tea.Cmd
	if !a.isStreaming && content == "" {
		a.isStreaming = true
		// Create spinner with label "Thinking..."
		spinner := NewGradientSpinner("#cba6f7", "#89b4fa", "Thinking...")
		a.spinner = &spinner
		spinnerCmd = a.spinner.Tick()
	}

	// Stop spinner when actual content arrives
	if a.isStreaming && content != "" {
		a.isStreaming = false
		a.spinner = nil
	}

	// If last message is a ThinkingMessageItem, append to it
	if len(a.messages) > 0 {
		if thinkingMsg, ok := a.messages[len(a.messages)-1].(*ThinkingMessageItem); ok {
			thinkingMsg.content += content
			// Invalidate cache
			thinkingMsg.cachedWidth = 0
			a.refreshContent()
			return spinnerCmd
		}
	}

	// Create new ThinkingMessageItem
	newMsg := &ThinkingMessageItem{
		id:        fmt.Sprintf("thinking-%d", len(a.messages)),
		content:   content,
		collapsed: true, // default true
	}
	a.messages = append(a.messages, newMsg)
	a.refreshContent()
	return spinnerCmd
}

// AppendFinish marks the iteration as finished and displays completion metadata.
// Sets finished=true on last ThinkingMessageItem (with duration), appends InfoMessageItem
// for model/provider/duration, and appends styled finish reason for errors/cancellations.
func (a *AgentOutput) AppendFinish(msg AgentFinishMsg) tea.Cmd {
	// Stop spinner when finish event received
	if a.isStreaming {
		a.isStreaming = false
		a.spinner = nil
	}

	// 1. Mark last ThinkingMessageItem as finished with duration
	if len(a.messages) > 0 {
		if thinkingMsg, ok := a.messages[len(a.messages)-1].(*ThinkingMessageItem); ok {
			thinkingMsg.finished = true
			thinkingMsg.duration = msg.Duration
			// Invalidate cache to trigger re-render with footer
			thinkingMsg.cachedWidth = 0
		}
	}

	// 1.5. If canceled, mark all pending/running tools as canceled
	if msg.Reason == "cancelled" {
		for _, message := range a.messages {
			if toolMsg, ok := message.(*ToolMessageItem); ok {
				if toolMsg.status == ToolStatusPending || toolMsg.status == ToolStatusRunning {
					toolMsg.status = ToolStatusCanceled
					// Invalidate cache
					toolMsg.cachedWidth = 0
				}
			}
		}
	}

	// 2. Append InfoMessageItem with model/provider/duration
	infoMsg := &InfoMessageItem{
		id:       fmt.Sprintf("info-%d", len(a.messages)),
		model:    msg.Model,
		provider: msg.Provider,
		duration: msg.Duration,
	}
	a.messages = append(a.messages, infoMsg)

	// 3. If error or canceled, append styled finish reason
	if msg.Error != "" {
		// Error finish
		errorText := styleFinishError.Render(fmt.Sprintf("Error: %s", msg.Error))
		errorItem := &TextMessageItem{
			id:      fmt.Sprintf("finish-error-%d", len(a.messages)),
			content: errorText,
		}
		a.messages = append(a.messages, errorItem)
	} else if msg.Reason == "cancelled" {
		// Canceled finish
		cancelText := styleFinishCanceled.Render("Iteration canceled")
		cancelItem := &TextMessageItem{
			id:      fmt.Sprintf("finish-cancel-%d", len(a.messages)),
			content: cancelText,
		}
		a.messages = append(a.messages, cancelItem)
	}

	a.refreshContent()
	return nil
}

// MarkToolError marks a tool call as failed with an error message.
// Finds the tool by ID, sets status to ToolStatusError, and updates output with error message.
func (a *AgentOutput) MarkToolError(toolCallID, errMsg string) tea.Cmd {
	idx, exists := a.toolIndex[toolCallID]
	if !exists {
		return nil
	}

	if toolMsg, ok := a.messages[idx].(*ToolMessageItem); ok {
		toolMsg.status = ToolStatusError
		toolMsg.output = errMsg
		// Invalidate cache
		toolMsg.cachedWidth = 0
		a.refreshContent()
	}

	return nil
}

// MarkToolCanceled marks a tool call as canceled.
// Finds the tool by ID and sets status to ToolStatusCanceled.
func (a *AgentOutput) MarkToolCanceled(toolCallID string) tea.Cmd {
	idx, exists := a.toolIndex[toolCallID]
	if !exists {
		return nil
	}

	if toolMsg, ok := a.messages[idx].(*ToolMessageItem); ok {
		toolMsg.status = ToolStatusCanceled
		// Invalidate cache
		toolMsg.cachedWidth = 0
		a.refreshContent()
	}

	return nil
}

// Append adds content to the agent output stream (legacy - calls AppendText).
func (a *AgentOutput) Append(content string) tea.Cmd {
	return a.AppendText(content)
}

// moveFocusPrevious moves focus to the previous expandable message.
// Wraps around to the last expandable message if at the beginning.
func (a *AgentOutput) moveFocusPrevious() {
	if len(a.messages) == 0 {
		return
	}

	// Find expandable messages
	expandableIndices := a.findExpandableIndices()
	if len(expandableIndices) == 0 {
		return
	}

	// If no message is focused, focus the last expandable message
	if a.focusedIndex < 0 {
		a.focusedIndex = expandableIndices[len(expandableIndices)-1]
		return
	}

	// Find current position in expandable list
	currentPos := -1
	for i, idx := range expandableIndices {
		if idx == a.focusedIndex {
			currentPos = i
			break
		}
	}

	// Move to previous expandable message (wrap around if needed)
	if currentPos > 0 {
		a.focusedIndex = expandableIndices[currentPos-1]
	} else {
		// Wrap to last expandable message
		a.focusedIndex = expandableIndices[len(expandableIndices)-1]
	}
}

// moveFocusNext moves focus to the next expandable message.
// Wraps around to the first expandable message if at the end.
func (a *AgentOutput) moveFocusNext() {
	if len(a.messages) == 0 {
		return
	}

	// Find expandable messages
	expandableIndices := a.findExpandableIndices()
	if len(expandableIndices) == 0 {
		return
	}

	// If no message is focused, focus the first expandable message
	if a.focusedIndex < 0 {
		a.focusedIndex = expandableIndices[0]
		return
	}

	// Find current position in expandable list
	currentPos := -1
	for i, idx := range expandableIndices {
		if idx == a.focusedIndex {
			currentPos = i
			break
		}
	}

	// Move to next expandable message (wrap around if needed)
	if currentPos >= 0 && currentPos < len(expandableIndices)-1 {
		a.focusedIndex = expandableIndices[currentPos+1]
	} else {
		// Wrap to first expandable message
		a.focusedIndex = expandableIndices[0]
	}
}

// findExpandableIndices returns the indices of all messages that implement Expandable.
func (a *AgentOutput) findExpandableIndices() []int {
	var indices []int
	for i, msg := range a.messages {
		if _, ok := msg.(Expandable); ok {
			indices = append(indices, i)
		}
	}
	return indices
}
