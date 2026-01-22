package agent

import (
	"encoding/json"
	"testing"
)

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name          string
		jsonLine      string
		expectText    string
		expectToolUse string
		expectError   string
	}{
		{
			name:       "text event",
			jsonLine:   `{"type":"text","content":"Hello, world!"}`,
			expectText: "Hello, world!",
		},
		{
			name:          "tool_use event",
			jsonLine:      `{"type":"tool_use","content":{"name":"task-add","input":{"content":"Test task","status":"remaining"}}}`,
			expectToolUse: "task-add",
		},
		{
			name:        "error event",
			jsonLine:    `{"type":"error","content":"Something went wrong"}`,
			expectError: "Something went wrong",
		},
		{
			name:     "unknown event type",
			jsonLine: `{"type":"unknown","content":"ignored"}`,
		},
		{
			name:     "invalid JSON",
			jsonLine: `{invalid json}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotText string
			var gotToolName string
			var gotError error

			runner := &Runner{
				onText: func(text string) {
					gotText = text
				},
				onToolUse: func(name string, input map[string]any) {
					gotToolName = name
				},
				onError: func(err error) {
					gotError = err
				},
			}

			runner.parseEvent(tt.jsonLine)

			if tt.expectText != "" && gotText != tt.expectText {
				t.Errorf("expected text %q, got %q", tt.expectText, gotText)
			}
			if tt.expectToolUse != "" && gotToolName != tt.expectToolUse {
				t.Errorf("expected tool use %q, got %q", tt.expectToolUse, gotToolName)
			}
			if tt.expectError != "" && (gotError == nil || gotError.Error() != tt.expectError) {
				t.Errorf("expected error %q, got %v", tt.expectError, gotError)
			}
		})
	}
}

func TestParseEventToolUseInput(t *testing.T) {
	var gotName string
	var gotInput map[string]any

	runner := &Runner{
		onToolUse: func(name string, input map[string]any) {
			gotName = name
			gotInput = input
		},
	}

	jsonLine := `{"type":"tool_use","content":{"name":"task-add","input":{"content":"Test task","status":"remaining"}}}`
	runner.parseEvent(jsonLine)

	if gotName != "task-add" {
		t.Errorf("expected name %q, got %q", "task-add", gotName)
	}

	if content, ok := gotInput["content"].(string); !ok || content != "Test task" {
		t.Errorf("expected content %q, got %v", "Test task", gotInput["content"])
	}

	if status, ok := gotInput["status"].(string); !ok || status != "remaining" {
		t.Errorf("expected status %q, got %v", "remaining", gotInput["status"])
	}
}

func TestParseEventEmptyLine(t *testing.T) {
	called := false
	runner := &Runner{
		onText: func(text string) {
			called = true
		},
	}

	// Empty lines should be handled gracefully
	runner.parseEvent("")

	if called {
		t.Error("expected no callbacks for empty line")
	}
}

func TestJSONMarshaling(t *testing.T) {
	// Test that our expected JSON structure matches what opencode produces
	type Event struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}

	// Text event
	textEvent := Event{
		Type:    "text",
		Content: json.RawMessage(`"Hello"`),
	}
	data, err := json.Marshal(textEvent)
	if err != nil {
		t.Fatalf("failed to marshal text event: %v", err)
	}
	if string(data) != `{"type":"text","content":"Hello"}` {
		t.Errorf("unexpected JSON: %s", data)
	}

	// Tool use event
	toolContent := struct {
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	}{
		Name:  "task-add",
		Input: map[string]any{"content": "Test"},
	}
	toolContentJSON, _ := json.Marshal(toolContent)
	toolEvent := Event{
		Type:    "tool_use",
		Content: json.RawMessage(toolContentJSON),
	}
	data, err = json.Marshal(toolEvent)
	if err != nil {
		t.Fatalf("failed to marshal tool event: %v", err)
	}

	// Verify it unmarshals correctly
	var parsed Event
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal tool event: %v", err)
	}
	if parsed.Type != "tool_use" {
		t.Errorf("expected type %q, got %q", "tool_use", parsed.Type)
	}
}
