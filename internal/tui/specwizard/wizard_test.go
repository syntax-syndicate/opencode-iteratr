package specwizard

import (
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
