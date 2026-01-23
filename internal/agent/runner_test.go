package agent

import "testing"

func TestExtractProvider(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "anthropic model",
			model:    "anthropic/claude-sonnet-4-5",
			expected: "Anthropic",
		},
		{
			name:     "openai model",
			model:    "openai/gpt-4",
			expected: "Openai",
		},
		{
			name:     "model without slash",
			model:    "claude-sonnet-4-5",
			expected: "",
		},
		{
			name:     "empty string",
			model:    "",
			expected: "",
		},
		{
			name:     "single letter provider",
			model:    "a/model",
			expected: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractProvider(tt.model)
			if got != tt.expected {
				t.Errorf("extractProvider(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}
