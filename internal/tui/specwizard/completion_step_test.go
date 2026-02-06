package specwizard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/iteratr/internal/tui/wizard"
)

// Helper function to create a KeyPressMsg from a string
func keyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: s})
}

func TestCompletionStep_New(t *testing.T) {
	specPath := "./specs/my-feature.md"
	step := NewCompletionStep(specPath)

	if step.specPath != specPath {
		t.Errorf("Expected specPath %q, got %q", specPath, step.specPath)
	}

	if step.buttonBar == nil {
		t.Error("Expected buttonBar to be initialized")
	}

	if !step.buttonFocused {
		t.Error("Expected buttonFocused to be true on creation")
	}
}

func TestCompletionStep_Init(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")
	cmd := step.Init()

	// Init should return nil (button focus happens in constructor)
	if cmd != nil {
		t.Error("Expected Init to return nil")
	}
}

func TestCompletionStep_View(t *testing.T) {
	specPath := "./specs/my-feature.md"
	step := NewCompletionStep(specPath)
	step.SetSize(80, 24)

	view := step.View()

	// Check for key elements in the view
	if !strings.Contains(view, "✓") || !strings.Contains(view, "Successfully") {
		t.Error("Expected success message with checkmark")
	}

	if !strings.Contains(view, specPath) {
		t.Errorf("Expected view to contain spec path %q", specPath)
	}

	if !strings.Contains(view, "Start Build") {
		t.Error("Expected view to contain 'Start Build' button")
	}

	if !strings.Contains(view, "Exit") {
		t.Error("Expected view to contain 'Exit' button")
	}

	if !strings.Contains(view, "What would you like to do next?") {
		t.Error("Expected view to contain instructions")
	}
}

func TestCompletionStep_KeyboardNavigation(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")
	step.Init()

	// Initial state: first button should be focused
	if step.buttonBar.FocusedButton() != wizard.ButtonBack {
		t.Error("Expected first button (Start Build) to be focused initially")
	}

	// Press Tab to move to second button
	cmd := step.Update(keyPress("tab"))
	if cmd != nil {
		t.Error("Expected tab navigation to not return a command")
	}

	if step.buttonBar.FocusedButton() != wizard.ButtonNext {
		t.Error("Expected second button (Exit) to be focused after Tab")
	}

	// Press Tab again to wrap around to first button
	step.Update(keyPress("tab"))
	if step.buttonBar.FocusedButton() != wizard.ButtonBack {
		t.Error("Expected focus to wrap around to first button")
	}

	// Press Shift+Tab to move backward
	step.Update(keyPress("shift+tab"))
	if step.buttonBar.FocusedButton() != wizard.ButtonNext {
		t.Error("Expected focus to move backward with Shift+Tab")
	}
}

func TestCompletionStep_StartBuildButton(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")
	step.Init()

	// Focus first button (Start Build)
	step.buttonBar.FocusFirst()

	// Press Enter to activate Start Build button
	cmd := step.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("Expected Enter on Start Build to return a command")
	}

	// Execute the command to get the message
	msg := cmd()
	if _, ok := msg.(StartBuildMsg); !ok {
		t.Errorf("Expected StartBuildMsg, got %T", msg)
	}
}

func TestCompletionStep_ExitButton(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")
	step.Init()

	// Focus second button (Exit)
	step.buttonBar.FocusFirst()
	step.Update(keyPress("tab"))

	// Press Enter to activate Exit button
	cmd := step.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("Expected Enter on Exit to return a command")
	}

	// The command should be tea.Quit
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected tea.QuitMsg from Exit button, got %T", msg)
	}
}

func TestCompletionStep_SetSize(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")

	width, height := 100, 30
	step.SetSize(width, height)

	if step.width != width {
		t.Errorf("Expected width %d, got %d", width, step.width)
	}

	if step.height != height {
		t.Errorf("Expected height %d, got %d", height, step.height)
	}
}

func TestCompletionStep_ArrowKeyNavigation(t *testing.T) {
	step := NewCompletionStep("./specs/test.md")
	step.Init()

	// Initial state: first button focused
	if step.buttonBar.FocusedButton() != wizard.ButtonBack {
		t.Error("Expected first button to be focused initially")
	}

	// Press Right arrow to move to second button
	step.Update(keyPress("right"))
	if step.buttonBar.FocusedButton() != wizard.ButtonNext {
		t.Error("Expected second button to be focused after Right arrow")
	}

	// Press Left arrow to move back to first button
	step.Update(keyPress("left"))
	if step.buttonBar.FocusedButton() != wizard.ButtonBack {
		t.Error("Expected first button to be focused after Left arrow")
	}
}
