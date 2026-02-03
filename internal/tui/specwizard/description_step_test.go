package specwizard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Helper function to create a KeyPressMsg from a string
func keyPressDesc(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: s})
}

func TestNewDescriptionStep(t *testing.T) {
	step := NewDescriptionStep()
	if step == nil {
		t.Fatal("NewDescriptionStep() returned nil")
	}
	if step.textarea.Value() != "" {
		t.Error("Expected empty textarea value")
	}
}

func TestDescriptionStep_Init(t *testing.T) {
	step := NewDescriptionStep()
	cmd := step.Init()
	if cmd == nil {
		t.Error("Init() should return blink command")
	}
}

func TestDescriptionStep_SetSize(t *testing.T) {
	step := NewDescriptionStep()
	step.SetSize(100, 50)
	if step.width != 100 {
		t.Errorf("Expected width 100, got %d", step.width)
	}
	if step.height != 50 {
		t.Errorf("Expected height 50, got %d", step.height)
	}
}

func TestDescriptionStep_FocusBlur(t *testing.T) {
	step := NewDescriptionStep()

	// Initial state should be focused
	step.Focus()
	if !step.textarea.Focused() {
		t.Error("Expected textarea to be focused")
	}

	// Test blur
	step.Blur()
	if step.textarea.Focused() {
		t.Error("Expected textarea to be blurred")
	}
}

func TestDescriptionStep_View(t *testing.T) {
	step := NewDescriptionStep()
	step.SetSize(80, 40)

	view := step.View()
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Check for key elements in the view
	if !strings.Contains(view, "Provide a detailed description") {
		t.Error("View should contain instruction text")
	}
	if !strings.Contains(view, "Ctrl+D") {
		t.Error("View should contain hint about Ctrl+D")
	}
}

func TestDescriptionStep_Submit_Empty(t *testing.T) {
	step := NewDescriptionStep()

	cmd := step.Submit()
	if cmd != nil {
		t.Error("Submit with empty description should not return command")
	}
	if step.err == "" {
		t.Error("Expected validation error for empty description")
	}
}

func TestDescriptionStep_Submit_TooShort(t *testing.T) {
	step := NewDescriptionStep()

	// Set a very short description
	step.textarea.SetValue("short")
	cmd := step.Submit()
	if cmd != nil {
		t.Error("Submit with short description should not return command")
	}
	if step.err == "" {
		t.Error("Expected validation error for short description")
	}
}

func TestDescriptionStep_Submit_Valid(t *testing.T) {
	step := NewDescriptionStep()

	// Set a valid description
	validDesc := "This is a valid description that is long enough to pass validation."
	step.textarea.SetValue(validDesc)

	cmd := step.Submit()
	if cmd == nil {
		t.Fatal("Submit with valid description should return command")
	}

	// Execute the command to get the message
	msg := cmd()
	descMsg, ok := msg.(DescriptionSubmittedMsg)
	if !ok {
		t.Fatalf("Expected DescriptionSubmittedMsg, got %T", msg)
	}
	if descMsg.Description != validDesc {
		t.Errorf("Expected description %q, got %q", validDesc, descMsg.Description)
	}

	// Error should be cleared
	if step.err != "" {
		t.Error("Error should be cleared after successful submit")
	}
}

func TestDescriptionStep_CtrlD_Submit(t *testing.T) {
	step := NewDescriptionStep()
	step.textarea.SetValue("This is a valid description for testing ctrl+d submission.")

	// Simulate Ctrl+D key press
	cmd := step.Update(keyPressDesc("ctrl+d"))
	if cmd == nil {
		t.Fatal("Ctrl+D should trigger submit")
	}

	msg := cmd()
	descMsg, ok := msg.(DescriptionSubmittedMsg)
	if !ok {
		t.Fatalf("Expected DescriptionSubmittedMsg, got %T", msg)
	}
	if descMsg.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestDescriptionStep_CtrlD_ValidationError(t *testing.T) {
	step := NewDescriptionStep()
	step.textarea.SetValue("") // Empty description

	// Simulate Ctrl+D key press with invalid input
	cmd := step.Update(keyPressDesc("ctrl+d"))
	if cmd != nil {
		t.Error("Ctrl+D with invalid input should not return command")
	}
	if step.err == "" {
		t.Error("Expected validation error")
	}
}

func TestDescriptionStep_ErrorClearsOnInput(t *testing.T) {
	step := NewDescriptionStep()

	// Set an error
	step.err = "Some error"

	// Simulate typing a character
	step.Update(keyPressDesc("a"))

	// Error should be cleared
	if step.err != "" {
		t.Error("Error should be cleared on input")
	}
}

func TestDescriptionStep_GetDescription(t *testing.T) {
	step := NewDescriptionStep()
	testDesc := "  Test description with spaces  "
	step.textarea.SetValue(testDesc)

	result := step.GetDescription()
	expected := strings.TrimSpace(testDesc)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestValidateDescription(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty description",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "short",
			wantErr: true,
		},
		{
			name:    "minimum valid length",
			input:   "1234567890",
			wantErr: false,
		},
		{
			name:    "valid description",
			input:   "This is a valid description with enough characters.",
			wantErr: false,
		},
		{
			name:    "maximum valid length",
			input:   strings.Repeat("a", 5000),
			wantErr: false,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 5001),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDescription(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDescription() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDescriptionStep_WindowSizeMsg(t *testing.T) {
	step := NewDescriptionStep()

	cmd := step.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	if cmd != nil {
		t.Error("WindowSizeMsg should not return command")
	}
	if step.width != 120 {
		t.Errorf("Expected width 120, got %d", step.width)
	}
	if step.height != 60 {
		t.Errorf("Expected height 60, got %d", step.height)
	}
}
