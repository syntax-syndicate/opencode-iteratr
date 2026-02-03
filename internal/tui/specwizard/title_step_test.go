package specwizard

import (
	"strings"
	"testing"
)

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid title",
			input:   "User Authentication",
			wantErr: false,
		},
		{
			name:    "valid title with extra spaces",
			input:   "  User Authentication  ",
			wantErr: false,
		},
		{
			name:    "empty title",
			input:   "",
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name:    "title too long",
			input:   strings.Repeat("a", 101),
			wantErr: true,
			errMsg:  "title too long (max 100 characters)",
		},
		{
			name:    "title exactly 100 chars",
			input:   strings.Repeat("a", 100),
			wantErr: false,
		},
		{
			name:    "title with special characters",
			input:   "API Rate-Limiting (v2)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTitle(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateTitle() expected error, got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateTitle() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateTitle() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTitleStep_GetTitle(t *testing.T) {
	step := NewTitleStep()

	// Initially empty
	if title := step.GetTitle(); title != "" {
		t.Errorf("GetTitle() = %q, want empty string", title)
	}

	// Set a value with spaces
	step.input.SetValue("  Test Title  ")
	title := step.GetTitle()

	if title != "Test Title" {
		t.Errorf("GetTitle() = %q, want %q", title, "Test Title")
	}
}
