package hooks

import (
	"context"
	"testing"
)

func TestExecuteAllPiped(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()
	vars := Variables{Session: "test", Iteration: "1"}

	tests := []struct {
		name     string
		hooks    []*HookConfig
		expected string
	}{
		{
			name:     "no hooks",
			hooks:    []*HookConfig{},
			expected: "",
		},
		{
			name: "single hook with pipe_output true",
			hooks: []*HookConfig{
				{Command: "echo 'piped'", Timeout: 5, PipeOutput: true},
			},
			expected: "piped\n",
		},
		{
			name: "single hook with pipe_output false",
			hooks: []*HookConfig{
				{Command: "echo 'not piped'", Timeout: 5, PipeOutput: false},
			},
			expected: "",
		},
		{
			name: "multiple hooks mixed pipe_output",
			hooks: []*HookConfig{
				{Command: "echo 'first piped'", Timeout: 5, PipeOutput: true},
				{Command: "echo 'not piped'", Timeout: 5, PipeOutput: false},
				{Command: "echo 'second piped'", Timeout: 5, PipeOutput: true},
			},
			expected: "first piped\n\nsecond piped\n",
		},
		{
			name: "all hooks with pipe_output false",
			hooks: []*HookConfig{
				{Command: "echo 'first'", Timeout: 5, PipeOutput: false},
				{Command: "echo 'second'", Timeout: 5, PipeOutput: false},
			},
			expected: "",
		},
		{
			name: "all hooks with pipe_output true",
			hooks: []*HookConfig{
				{Command: "echo 'first'", Timeout: 5, PipeOutput: true},
				{Command: "echo 'second'", Timeout: 5, PipeOutput: true},
			},
			expected: "first\n\nsecond\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := ExecuteAllPiped(ctx, tt.hooks, workDir, vars)
			if err != nil {
				t.Fatalf("ExecuteAllPiped() error = %v", err)
			}
			if output != tt.expected {
				t.Errorf("ExecuteAllPiped() output = %q, expected %q", output, tt.expected)
			}
		})
	}
}

func TestExecuteAllPiped_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	workDir := t.TempDir()
	vars := Variables{Session: "test", Iteration: "1"}
	hooks := []*HookConfig{
		{Command: "echo 'test'", Timeout: 5, PipeOutput: true},
	}

	_, err := ExecuteAllPiped(ctx, hooks, workDir, vars)
	if err == nil {
		t.Error("ExecuteAllPiped() expected error for cancelled context, got nil")
	}
}
