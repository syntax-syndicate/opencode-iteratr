package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewAgentOutput(t *testing.T) {
	ao := NewAgentOutput()

	if ao == nil {
		t.Fatal("expected non-nil agent output")
	}
	// Note: autoScroll is now managed by ScrollList after UpdateSize is called
}

func TestAgentOutput_Append(t *testing.T) {
	ao := NewAgentOutput()

	// Append some content
	cmd := ao.Append("Test content")

	// Command can be nil - just verify it doesn't panic
	_ = cmd
}

func TestAgentOutput_UpdateSize(t *testing.T) {
	ao := NewAgentOutput()

	cmd := ao.UpdateSize(100, 50)

	// Command can be nil - just verify it doesn't panic
	_ = cmd

	if ao.width != 100 {
		t.Errorf("width: got %d, want 100", ao.width)
	}
	if ao.height != 50 {
		t.Errorf("height: got %d, want 50", ao.height)
	}
	if !ao.ready {
		t.Error("expected viewport to be ready after UpdateSize")
	}
}

func TestAgentOutput_Render(t *testing.T) {
	ao := NewAgentOutput()

	output := ao.Render()

	// Should render something even with no content
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestAgentOutput_ToggleInvalidatesCacheAndRefreshes(t *testing.T) {
	ao := NewAgentOutput()
	// Don't set ready manually - UpdateSize will initialize scrollList and set ready
	ao.UpdateSize(80, 20)

	// Add a tool message with long output (more than maxLines)
	toolMsg := AgentToolCallMsg{
		ToolCallID: "tool-1",
		Title:      "Read",
		Status:     "completed",
		Input:      map[string]any{"filePath": "test.go"},
		Output:     "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12",
	}
	ao.AppendToolCall(toolMsg)

	// Set focus on this tool message
	ao.focusedIndex = 0

	toolItem := ao.messages[0].(*ToolMessageItem)

	// Render at initial width to populate cache
	toolItem.Render(76) // contentWidth = width - 4 = 80 - 4 = 76
	initialCachedWidth := toolItem.cachedWidth
	initialCachedRender := toolItem.cachedRender

	if initialCachedWidth != 76 {
		t.Errorf("expected cachedWidth to be 76, got %d", initialCachedWidth)
	}
	if initialCachedRender == "" {
		t.Fatal("expected cachedRender to be populated")
	}

	// Verify message is not expanded initially
	if toolItem.IsExpanded() {
		t.Fatal("expected tool message to not be expanded initially")
	}

	// Toggle expansion with space key
	keyMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	ao.Update(keyMsg)

	// Message should now be expanded
	if !toolItem.IsExpanded() {
		t.Error("expected tool message to be expanded after toggle")
	}

	// With ScrollList, items are rendered lazily when View() is called
	// So we need to call View() to trigger rendering and populate the cache
	scrollListContent := ao.scrollList.View()

	// The scroll list should have content (not empty)
	if scrollListContent == "" {
		t.Error("expected scroll list content to be non-empty after refresh")
	}

	// After View() is called, cache should be populated
	// contentWidth = width(80) - 5 (border, padding, left margin)
	if toolItem.cachedWidth != 75 {
		t.Errorf("expected cachedWidth to be refreshed to 75, got %d", toolItem.cachedWidth)
	}

	// The cached render should be different from initial (because expansion state changed)
	newCachedRender := toolItem.cachedRender
	if newCachedRender == initialCachedRender {
		t.Error("expected cachedRender to change after toggle (different expansion state)")
	}

	// The new cached render should contain more lines (expanded shows all output)
	if !containsSubstring(newCachedRender, "line11") || !containsSubstring(newCachedRender, "line12") {
		t.Error("expected expanded output to contain all lines including line11 and line12")
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAgentOutput_UpDownKeyHandling(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add a mix of message types: text, thinking, tool, text, thinking, tool
	ao.AppendText("First text message")

	ao.AppendThinking("First thinking block")

	toolMsg1 := AgentToolCallMsg{
		ToolCallID: "tool-1",
		Title:      "Read",
		Status:     "completed",
		Input:      map[string]any{"filePath": "test.go"},
		Output:     "tool output 1",
	}
	ao.AppendToolCall(toolMsg1)

	ao.AppendText("Second text message")

	ao.AppendThinking("Second thinking block")

	toolMsg2 := AgentToolCallMsg{
		ToolCallID: "tool-2",
		Title:      "Edit",
		Status:     "completed",
		Input:      map[string]any{"filePath": "test.go"},
		Output:     "tool output 2",
	}
	ao.AppendToolCall(toolMsg2)

	// Messages are at indices: 0=text, 1=thinking, 2=tool, 3=text, 4=thinking, 5=tool
	// Expandable messages are at indices: 1, 2, 4, 5

	// Initially no message is focused
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to be -1, got %d", ao.focusedIndex)
	}

	// Press Down arrow - should focus first expandable message (index 1)
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	ao.Update(downKey)
	if ao.focusedIndex != 1 {
		t.Errorf("expected focusedIndex to be 1 after first down, got %d", ao.focusedIndex)
	}

	// Press Down arrow again - should move to next expandable (index 2)
	ao.Update(downKey)
	if ao.focusedIndex != 2 {
		t.Errorf("expected focusedIndex to be 2 after second down, got %d", ao.focusedIndex)
	}

	// Press Down arrow again - should move to next expandable (index 4)
	ao.Update(downKey)
	if ao.focusedIndex != 4 {
		t.Errorf("expected focusedIndex to be 4 after third down, got %d", ao.focusedIndex)
	}

	// Press Down arrow again - should move to next expandable (index 5)
	ao.Update(downKey)
	if ao.focusedIndex != 5 {
		t.Errorf("expected focusedIndex to be 5 after fourth down, got %d", ao.focusedIndex)
	}

	// Press Down arrow again - should wrap to first expandable (index 1)
	ao.Update(downKey)
	if ao.focusedIndex != 1 {
		t.Errorf("expected focusedIndex to wrap to 1 after fifth down, got %d", ao.focusedIndex)
	}

	// Press Up arrow - should wrap to last expandable (index 5)
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	ao.Update(upKey)
	if ao.focusedIndex != 5 {
		t.Errorf("expected focusedIndex to wrap to 5 after up from first, got %d", ao.focusedIndex)
	}

	// Press Up arrow again - should move to previous expandable (index 4)
	ao.Update(upKey)
	if ao.focusedIndex != 4 {
		t.Errorf("expected focusedIndex to be 4 after up, got %d", ao.focusedIndex)
	}

	// Press Up arrow again - should move to previous expandable (index 2)
	ao.Update(upKey)
	if ao.focusedIndex != 2 {
		t.Errorf("expected focusedIndex to be 2 after up, got %d", ao.focusedIndex)
	}

	// Press Up arrow again - should move to previous expandable (index 1)
	ao.Update(upKey)
	if ao.focusedIndex != 1 {
		t.Errorf("expected focusedIndex to be 1 after up, got %d", ao.focusedIndex)
	}

	// Press Up arrow again - should wrap to last expandable (index 5)
	ao.Update(upKey)
	if ao.focusedIndex != 5 {
		t.Errorf("expected focusedIndex to wrap to 5 after up from first, got %d", ao.focusedIndex)
	}
}

func TestAgentOutput_UpDownKeyHandling_NoExpandableMessages(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add only text messages (not expandable)
	ao.AppendText("First text message")
	ao.AppendText("Second text message")

	// Initially no message is focused
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to be -1, got %d", ao.focusedIndex)
	}

	// Press Down arrow - should not change focus (no expandable messages)
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	ao.Update(downKey)
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to remain -1, got %d", ao.focusedIndex)
	}

	// Press Up arrow - should not change focus (no expandable messages)
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	ao.Update(upKey)
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to remain -1, got %d", ao.focusedIndex)
	}
}

func TestAgentOutput_UpDownKeyHandling_EmptyMessages(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// No messages at all
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to be -1, got %d", ao.focusedIndex)
	}

	// Press Down arrow - should not change focus (no messages)
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	ao.Update(downKey)
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to remain -1, got %d", ao.focusedIndex)
	}

	// Press Up arrow - should not change focus (no messages)
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	ao.Update(upKey)
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to remain -1, got %d", ao.focusedIndex)
	}
}

func TestAgentOutput_UpDownKeyHandling_SingleExpandableMessage(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add only one expandable message
	ao.AppendThinking("Single thinking block")

	// Initially no message is focused
	if ao.focusedIndex != -1 {
		t.Errorf("expected focusedIndex to be -1, got %d", ao.focusedIndex)
	}

	// Press Down arrow - should focus the only expandable message (index 0)
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	ao.Update(downKey)
	if ao.focusedIndex != 0 {
		t.Errorf("expected focusedIndex to be 0, got %d", ao.focusedIndex)
	}

	// Press Down arrow again - should stay at index 0 (wrap to itself)
	ao.Update(downKey)
	if ao.focusedIndex != 0 {
		t.Errorf("expected focusedIndex to remain at 0, got %d", ao.focusedIndex)
	}

	// Press Up arrow - should stay at index 0 (wrap to itself)
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	ao.Update(upKey)
	if ao.focusedIndex != 0 {
		t.Errorf("expected focusedIndex to remain at 0, got %d", ao.focusedIndex)
	}
}

func TestAgentOutput_ToggleExpandedOnFocusedMessage(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add a tool message
	toolMsg := AgentToolCallMsg{
		ToolCallID: "tool-1",
		Title:      "Read",
		Status:     "completed",
		Input:      map[string]any{"filePath": "test.go"},
		Output:     "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12",
	}
	ao.AppendToolCall(toolMsg)

	// Focus the message with Down arrow
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	ao.Update(downKey)

	toolItem := ao.messages[0].(*ToolMessageItem)

	// Verify message is not expanded initially
	if toolItem.IsExpanded() {
		t.Fatal("expected tool message to not be expanded initially")
	}

	// Toggle expansion with space key
	spaceKey := tea.KeyPressMsg{Code: ' ', Text: " "}
	ao.Update(spaceKey)

	// Message should now be expanded
	if !toolItem.IsExpanded() {
		t.Error("expected tool message to be expanded after space")
	}

	// Toggle back with enter key
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}
	ao.Update(enterKey)

	// Message should now be collapsed
	if toolItem.IsExpanded() {
		t.Error("expected tool message to be collapsed after enter")
	}
}

func TestAgentOutput_AppendFinish_Normal(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add a thinking message
	ao.AppendThinking("Processing request...")

	// Initially the thinking message should not be finished
	thinkingMsg := ao.messages[0].(*ThinkingMessageItem)
	if thinkingMsg.finished {
		t.Error("expected thinking message to not be finished initially")
	}

	// Call AppendFinish with normal completion
	finishMsg := AgentFinishMsg{
		Reason:   "end_turn",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 5000000000, // 5 seconds in nanoseconds
	}
	ao.AppendFinish(finishMsg)

	// Verify thinking message is now finished with duration
	if !thinkingMsg.finished {
		t.Error("expected thinking message to be finished after AppendFinish")
	}
	if thinkingMsg.duration != finishMsg.Duration {
		t.Errorf("expected thinking duration to be %v, got %v", finishMsg.Duration, thinkingMsg.duration)
	}

	// Note: cache will be repopulated by refreshContent() at end of AppendFinish,
	// so we don't check cachedWidth=0 here. The important thing is that finished=true
	// and duration is set, which will affect the rendered output (footer with duration).

	// Verify InfoMessageItem was appended
	if len(ao.messages) < 2 {
		t.Fatal("expected at least 2 messages after AppendFinish")
	}
	infoMsg, ok := ao.messages[1].(*InfoMessageItem)
	if !ok {
		t.Fatal("expected second message to be InfoMessageItem")
	}
	if infoMsg.model != finishMsg.Model {
		t.Errorf("expected info model to be %s, got %s", finishMsg.Model, infoMsg.model)
	}
	if infoMsg.provider != finishMsg.Provider {
		t.Errorf("expected info provider to be %s, got %s", finishMsg.Provider, infoMsg.provider)
	}
	if infoMsg.duration != finishMsg.Duration {
		t.Errorf("expected info duration to be %v, got %v", finishMsg.Duration, infoMsg.duration)
	}

	// Verify no error or cancel messages were added
	if len(ao.messages) != 2 {
		t.Errorf("expected exactly 2 messages for normal finish, got %d", len(ao.messages))
	}
}

func TestAgentOutput_AppendFinish_WithError(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add a thinking message
	ao.AppendThinking("Processing request...")

	// Call AppendFinish with error
	finishMsg := AgentFinishMsg{
		Reason:   "error",
		Error:    "Connection timeout",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 2000000000, // 2 seconds
	}
	ao.AppendFinish(finishMsg)

	// Verify thinking message is finished
	thinkingMsg := ao.messages[0].(*ThinkingMessageItem)
	if !thinkingMsg.finished {
		t.Error("expected thinking message to be finished")
	}

	// Verify InfoMessageItem was appended
	if len(ao.messages) < 2 {
		t.Fatal("expected at least 2 messages")
	}
	_, ok := ao.messages[1].(*InfoMessageItem)
	if !ok {
		t.Fatal("expected second message to be InfoMessageItem")
	}

	// Verify error TextMessageItem was appended
	if len(ao.messages) < 3 {
		t.Fatal("expected at least 3 messages (thinking, info, error)")
	}
	errorMsg, ok := ao.messages[2].(*TextMessageItem)
	if !ok {
		t.Fatal("expected third message to be TextMessageItem for error")
	}
	if !containsSubstring(errorMsg.content, "Error") || !containsSubstring(errorMsg.content, "Connection timeout") {
		t.Errorf("expected error message to contain error text, got: %s", errorMsg.content)
	}

	// Total should be 3 messages: thinking, info, error
	if len(ao.messages) != 3 {
		t.Errorf("expected exactly 3 messages for error finish, got %d", len(ao.messages))
	}
}

func TestAgentOutput_AppendFinish_Canceled(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add a thinking message
	ao.AppendThinking("Processing request...")

	// Call AppendFinish with canceled
	finishMsg := AgentFinishMsg{
		Reason:   "cancelled",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 1000000000, // 1 second
	}
	ao.AppendFinish(finishMsg)

	// Verify thinking message is finished
	thinkingMsg := ao.messages[0].(*ThinkingMessageItem)
	if !thinkingMsg.finished {
		t.Error("expected thinking message to be finished")
	}

	// Verify InfoMessageItem was appended
	if len(ao.messages) < 2 {
		t.Fatal("expected at least 2 messages")
	}
	_, ok := ao.messages[1].(*InfoMessageItem)
	if !ok {
		t.Fatal("expected second message to be InfoMessageItem")
	}

	// Verify cancel TextMessageItem was appended
	if len(ao.messages) < 3 {
		t.Fatal("expected at least 3 messages (thinking, info, cancel)")
	}
	cancelMsg, ok := ao.messages[2].(*TextMessageItem)
	if !ok {
		t.Fatal("expected third message to be TextMessageItem for cancel")
	}
	if !containsSubstring(cancelMsg.content, "canceled") {
		t.Errorf("expected cancel message to contain 'canceled', got: %s", cancelMsg.content)
	}

	// Total should be 3 messages: thinking, info, cancel
	if len(ao.messages) != 3 {
		t.Errorf("expected exactly 3 messages for canceled finish, got %d", len(ao.messages))
	}
}

func TestAgentOutput_AppendFinish_StopsSpinner(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Start streaming (which starts spinner)
	ao.AppendThinking("Processing...")
	ao.isStreaming = true
	ao.spinner = &GradientSpinner{label: "Thinking..."}

	// Verify spinner is active
	if !ao.isStreaming {
		t.Error("expected streaming to be active")
	}
	if ao.spinner == nil {
		t.Error("expected spinner to be present")
	}

	// Call AppendFinish
	finishMsg := AgentFinishMsg{
		Reason:   "end_turn",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 3000000000,
	}
	ao.AppendFinish(finishMsg)

	// Verify spinner was stopped
	if ao.isStreaming {
		t.Error("expected streaming to be stopped after AppendFinish")
	}
	if ao.spinner != nil {
		t.Error("expected spinner to be nil after AppendFinish")
	}
}

func TestAgentOutput_AppendFinish_NoThinkingMessage(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// No thinking message - just call AppendFinish
	finishMsg := AgentFinishMsg{
		Reason:   "end_turn",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 5000000000,
	}
	ao.AppendFinish(finishMsg)

	// Should still append InfoMessageItem
	if len(ao.messages) < 1 {
		t.Fatal("expected at least 1 message (info)")
	}
	infoMsg, ok := ao.messages[0].(*InfoMessageItem)
	if !ok {
		t.Fatal("expected first message to be InfoMessageItem")
	}
	if infoMsg.model != finishMsg.Model {
		t.Errorf("expected info model to be %s, got %s", finishMsg.Model, infoMsg.model)
	}
}

func TestAgentOutput_AppendFinish_CancelsPendingTools(t *testing.T) {
	ao := NewAgentOutput()
	ao.ready = true
	ao.UpdateSize(80, 20)

	// Add some tool calls in different states
	ao.AppendToolCall(AgentToolCallMsg{
		ToolCallID: "tool1",
		Title:      "bash",
		Status:     "pending",
		Input:      map[string]any{"command": "ls -la"},
	})
	ao.AppendToolCall(AgentToolCallMsg{
		ToolCallID: "tool2",
		Title:      "read",
		Status:     "in_progress",
		Input:      map[string]any{"filePath": "file.txt"},
	})
	ao.AppendToolCall(AgentToolCallMsg{
		ToolCallID: "tool3",
		Title:      "write",
		Status:     "completed",
		Input:      map[string]any{"filePath": "output.txt"},
		Output:     "success",
	})

	// Call AppendFinish with cancelled reason
	finishMsg := AgentFinishMsg{
		Reason:   "cancelled",
		Model:    "claude-sonnet-4-5",
		Provider: "Anthropic",
		Duration: 2000000000,
	}
	ao.AppendFinish(finishMsg)

	// Verify tool1 (pending) is now canceled
	tool1, ok := ao.messages[0].(*ToolMessageItem)
	if !ok {
		t.Fatal("expected first message to be ToolMessageItem")
	}
	if tool1.status != ToolStatusCanceled {
		t.Errorf("expected tool1 status to be canceled, got %d", tool1.status)
	}

	// Verify tool2 (in_progress) is now canceled
	tool2, ok := ao.messages[1].(*ToolMessageItem)
	if !ok {
		t.Fatal("expected second message to be ToolMessageItem")
	}
	if tool2.status != ToolStatusCanceled {
		t.Errorf("expected tool2 status to be canceled, got %d", tool2.status)
	}

	// Verify tool3 (completed) remains completed
	tool3, ok := ao.messages[2].(*ToolMessageItem)
	if !ok {
		t.Fatal("expected third message to be ToolMessageItem")
	}
	if tool3.status != ToolStatusSuccess {
		t.Errorf("expected tool3 status to remain success, got %d", tool3.status)
	}

	// Verify cancel message was appended
	foundCancelMsg := false
	for _, msg := range ao.messages {
		if textMsg, ok := msg.(*TextMessageItem); ok {
			if textMsg.content != "" && textMsg.id != "" {
				foundCancelMsg = true
				break
			}
		}
	}
	if !foundCancelMsg {
		t.Error("expected cancel message to be appended")
	}
}
