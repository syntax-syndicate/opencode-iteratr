package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/iteratr/internal/hooks"
)

func TestPostIterationHooks(t *testing.T) {
	t.Run("executes after IterationComplete", func(t *testing.T) {
		// Create temp directory for test
		tmpDir := t.TempDir()

		// Create hooks config
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "echo 'Post iteration hook executed'",
						Timeout:    5,
						PipeOutput: true,
					},
				},
			},
		}

		// Simulate iteration complete by executing the post-iteration logic
		ctx := context.Background()
		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "1",
		}
		output, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify output was returned (simulating appendPendingOutput)
		if output == "" {
			t.Error("Expected hook output, got empty string")
		}

		// Verify output contains expected message
		if output != "Post iteration hook executed\n" {
			t.Errorf("Expected 'Post iteration hook executed\\n', got %q", output)
		}
	})

	t.Run("only pipes output when pipe_output is true", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hooks config with mixed pipe_output settings
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "echo 'Piped output'",
						Timeout:    5,
						PipeOutput: true,
					},
					{
						Command:    "echo 'Side effect only'",
						Timeout:    5,
						PipeOutput: false, // Should not be included
					},
					{
						Command:    "echo 'Another piped output'",
						Timeout:    5,
						PipeOutput: true,
					},
				},
			},
		}

		ctx := context.Background()
		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "1",
		}
		output, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should only contain piped outputs (echo adds \n, join adds \n between)
		expected := "Piped output\n\nAnother piped output\n"
		if output != expected {
			t.Errorf("Expected %q, got %q", expected, output)
		}
	})

	t.Run("no output when pipe_output is false for all hooks", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hooks config with all pipe_output: false
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "echo 'Side effect 1'",
						Timeout:    5,
						PipeOutput: false,
					},
					{
						Command:    "echo 'Side effect 2'",
						Timeout:    5,
						PipeOutput: false,
					},
				},
			},
		}

		ctx := context.Background()
		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "1",
		}
		output, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should return empty string
		if output != "" {
			t.Errorf("Expected empty string, got %q", output)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hooks config with long-running command
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "sleep 10",
						Timeout:    30,
						PipeOutput: true,
					},
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "1",
		}
		_, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)

		// Should propagate context cancellation
		if err == nil {
			t.Error("Expected context cancellation error, got nil")
		}
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("template variables are expanded", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "hook_output.txt")

		// Create hooks config with template variables
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "echo 'Session: {{session}}, Iteration: {{iteration}}' > " + outputFile,
						Timeout:    5,
						PipeOutput: false, // Just checking side effect
					},
				},
			},
		}

		ctx := context.Background()
		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "42",
		}
		_, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Give file system a moment to sync
		time.Sleep(10 * time.Millisecond)

		// Read output file to verify template expansion
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		expected := "Session: test-session, Iteration: 42\n"
		if string(content) != expected {
			t.Errorf("Expected %q, got %q", expected, string(content))
		}
	})

	t.Run("appends to pending buffer in correct order", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hooks config
		hooksConfig := &hooks.Config{
			Version: 1,
			Hooks: hooks.HooksConfig{
				PostIteration: []*hooks.HookConfig{
					{
						Command:    "echo 'First'",
						Timeout:    5,
						PipeOutput: true,
					},
					{
						Command:    "echo 'Second'",
						Timeout:    5,
						PipeOutput: true,
					},
				},
			},
		}

		o := &Orchestrator{
			cfg:         Config{SessionName: "test-session", WorkDir: tmpDir},
			hooksConfig: hooksConfig,
			ctx:         context.Background(),
		}

		// Execute hooks
		ctx := context.Background()
		hookVars := hooks.Variables{
			Session:   "test-session",
			Iteration: "1",
		}
		output, err := hooks.ExecuteAllPiped(ctx, hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Simulate appending to pending buffer
		o.appendPendingOutput(output)

		// Verify FIFO order (echo adds \n, join adds \n between)
		drained := o.drainPendingOutput()
		expected := "First\n\nSecond\n"
		if drained != expected {
			t.Errorf("Expected %q, got %q", expected, drained)
		}
	})
}
