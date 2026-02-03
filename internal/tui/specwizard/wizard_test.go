package specwizard

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildSpecPrompt(t *testing.T) {
	title := "User Authentication"
	description := "Add user authentication with email/password login"

	prompt := buildSpecPrompt(title, description)

	// Verify title is included
	if !strings.Contains(prompt, title) {
		t.Errorf("Prompt does not contain title: %s", title)
	}

	// Verify description is included
	if !strings.Contains(prompt, description) {
		t.Errorf("Prompt does not contain description: %s", description)
	}

	// Verify key instructions are present
	expectedPhrases := []string{
		"You are helping create a feature specification",
		"using the ask-questions",
		"using the finish-spec tool",
		"## Overview",
		"## User Story",
		"## Requirements",
		"## Technical Implementation",
		"## Tasks",
		"## Out of Scope",
		"extremely concise",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("Prompt missing expected phrase: %s", phrase)
		}
	}
}

func TestAgentErrorHandling(t *testing.T) {
	tests := []struct {
		name            string
		errorMsg        string
		expectedContent []string
	}{
		{
			name:     "opencode not installed",
			errorMsg: "failed to start opencode: executable file not found in $PATH",
			expectedContent: []string{
				"⚠ Agent Startup Failed",
				"opencode is not installed",
				"npm install -g opencode",
				"opencode --version",
			},
		},
		{
			name:     "MCP server start failure",
			errorMsg: "failed to start MCP server: failed to find available port",
			expectedContent: []string{
				"⚠ Agent Startup Failed",
				"Failed to start internal MCP server",
				"No available ports",
				"Try restarting the wizard",
			},
		},
		{
			name:     "ACP initialization failure",
			errorMsg: "ACP initialize failed: protocol error",
			expectedContent: []string{
				"⚠ Agent Startup Failed",
				"Failed to initialize agent communication",
				"opencode version mismatch",
				"npm install -g opencode",
			},
		},
		{
			name:     "Generic error",
			errorMsg: "some unexpected error occurred",
			expectedContent: []string{
				"⚠ Agent Startup Failed",
				"An unexpected error occurred",
				"check the logs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create wizard model with agent error
			m := &WizardModel{
				step:   StepAgent,
				width:  80,
				height: 24,
			}

			// Set agent error
			errVar := fmt.Errorf("%s", tt.errorMsg)
			m.agentError = &errVar

			// Render error screen
			output := m.renderErrorScreen(errVar)

			// Verify expected content is present
			for _, expected := range tt.expectedContent {
				if !strings.Contains(output, expected) {
					t.Errorf("Error screen missing expected content: %q\nGot:\n%s", expected, output)
				}
			}

			// Verify error message text is included (may be formatted)
			// Error is shown as "Error: <message>" so check for the core message
			if !strings.Contains(output, "Error:") {
				t.Error("Error screen missing 'Error:' prefix")
			}
		})
	}
}

func TestAgentErrorMsg(t *testing.T) {
	// Create wizard model
	m := &WizardModel{
		step:   StepAgent,
		width:  80,
		height: 24,
	}

	// Send AgentErrorMsg
	err := fmt.Errorf("test error: opencode not found")
	updatedModel, _ := m.Update(AgentErrorMsg{Err: err})

	// Verify error was stored
	wizModel := updatedModel.(*WizardModel)
	if wizModel.agentError == nil {
		t.Error("Expected agentError to be set")
	}

	if *wizModel.agentError != err {
		t.Errorf("Expected agentError to be %v, got %v", err, *wizModel.agentError)
	}

	// Verify renderCurrentStep shows error screen
	output := wizModel.renderCurrentStep()
	if !strings.Contains(output, "⚠ Agent Startup Failed") {
		t.Error("Expected error screen to be rendered")
	}
	if !strings.Contains(output, "test error: opencode not found") {
		t.Error("Expected error message to be shown")
	}
}
