package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/iteratr/internal/tui/testfixtures"
)

var update = flag.Bool("update", false, "update golden files")

// TestMessageExpanded_Text tests TextMessageItem in expanded state (always expanded)
func TestMessageExpanded_Text(t *testing.T) {
	msg := &TextMessageItem{
		id: "text-1",
		content: `This is a text message from the assistant.

It can contain multiple paragraphs with **markdown** formatting.

Here's a code block:
` + "```go\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n```" + `

And some more text after the code.`,
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_text.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_User tests UserMessageItem in expanded state (always expanded)
func TestMessageExpanded_User(t *testing.T) {
	msg := &UserMessageItem{
		id:      "user-1",
		content: "This is a user message. It should be right-aligned with a different border style.",
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_user.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_Thinking tests ThinkingMessageItem in expanded state
func TestMessageExpanded_Thinking(t *testing.T) {
	// Create thinking message with many lines
	var content strings.Builder
	for i := 1; i <= 15; i++ {
		if i > 1 {
			content.WriteString("\n")
		}
		content.WriteString("Thinking line ")
		content.WriteString(string(rune('0' + i)))
		content.WriteString(": analyzing the problem and considering various approaches.")
	}

	msg := &ThinkingMessageItem{
		id:        "thinking-1",
		content:   content.String(),
		collapsed: false, // EXPANDED
		duration:  3500 * time.Millisecond,
		finished:  true,
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_thinking.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_Tool tests ToolMessageItem in expanded state
func TestMessageExpanded_Tool(t *testing.T) {
	// Create tool message with long output
	var output strings.Builder
	for i := 1; i <= 20; i++ {
		if i > 1 {
			output.WriteString("\n")
		}
		output.WriteString("Output line ")
		output.WriteString(string(rune('0' + (i % 10))))
		output.WriteString(": This is a line of tool output that demonstrates the expanded view.")
	}

	msg := &ToolMessageItem{
		id:       "tool-1",
		toolName: "Read",
		kind:     "read",
		status:   ToolStatusSuccess,
		input: map[string]any{
			"filePath": "/path/to/file.go",
		},
		output:   output.String(),
		expanded: true, // EXPANDED
		maxLines: 10,
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_tool.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_ToolWithDiff tests ToolMessageItem with file diff in expanded state
func TestMessageExpanded_ToolWithDiff(t *testing.T) {
	before := "package main\n\nfunc main() {\n\tprintln(\"old content\")\n}\n"
	after := "package main\n\nfunc main() {\n\tprintln(\"new content\")\n}\n"

	msg := &ToolMessageItem{
		id:       "tool-2",
		toolName: "Edit",
		kind:     "edit",
		status:   ToolStatusSuccess,
		input: map[string]any{
			"filePath":  "/path/to/modified.go",
			"oldString": "old content",
			"newString": "new content",
		},
		fileDiff: &FileDiff{
			File:      "/path/to/modified.go",
			Before:    before,
			After:     after,
			Additions: 1,
			Deletions: 1,
		},
		expanded: true, // EXPANDED
		maxLines: 10,
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_tool_diff.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_Info tests InfoMessageItem in expanded state (always expanded)
func TestMessageExpanded_Info(t *testing.T) {
	msg := &InfoMessageItem{
		id:       "info-1",
		model:    "claude-sonnet-4-5",
		provider: "Anthropic",
		duration: 5250 * time.Millisecond,
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_info.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_Subagent tests SubagentMessageItem in expanded state
func TestMessageExpanded_Subagent(t *testing.T) {
	msg := &SubagentMessageItem{
		id:           "subagent-1",
		subagentType: "codebase-analyzer",
		description:  "Analyze the codebase for patterns and usage examples",
		status:       ToolStatusSuccess,
		sessionID:    "session-abc123",
		// Note: SubagentMessageItem doesn't have expanded/maxLines fields
		// It's always fully rendered
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_subagent.golden")
	compareGolden(t, goldenFile, rendered)
}

// TestMessageExpanded_Divider tests DividerMessageItem in expanded state (always expanded)
func TestMessageExpanded_Divider(t *testing.T) {
	msg := &DividerMessageItem{
		id:        "divider-1",
		iteration: 3, // Show "Iteration #3" divider
	}

	// Render at test width
	rendered := msg.Render(testfixtures.TestTermWidth)

	// Verify golden file
	goldenFile := filepath.Join("testdata", "message_expanded_divider.golden")
	compareGolden(t, goldenFile, rendered)
}

// compareGolden compares rendered output with golden file
func compareGolden(t *testing.T, goldenPath, actual string) {
	t.Helper()

	// Update golden file if -update flag is set
	if *update {
		// Ensure testdata directory exists
		dir := filepath.Dir(goldenPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create testdata directory: %v", err)
		}

		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("failed to update golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read golden file
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file %s does not exist. Run with -update to create it.", goldenPath)
		}
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	// Compare
	if actual != string(expected) {
		t.Errorf("output does not match golden file %s\n\nExpected:\n%s\n\nActual:\n%s",
			goldenPath, string(expected), actual)
	}
}
