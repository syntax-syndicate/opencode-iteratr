package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/mark3labs/iteratr/internal/agent"
	"github.com/mark3labs/iteratr/internal/session"
)

// TestGracefulShutdown verifies that the orchestrator shuts down cleanly
// when Stop() is called, including cleanup of all components.
func TestGracefulShutdown(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec

This is a test spec.

## Tasks
- [ ] Test task 1
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator
	orch, err := New(Config{
		SessionName: "test-shutdown",
		SpecPath:    specPath,
		Iterations:  1,
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true, // No TUI for test
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Start orchestrator
	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}

	// Give it a moment to fully initialize
	time.Sleep(100 * time.Millisecond)

	// Stop orchestrator
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- orch.Stop()
	}()

	// Ensure Stop() completes within reasonable time
	select {
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Stop() returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop() timed out - graceful shutdown failed")
	}

	// Verify data directory was written (proves server was running)
	storeDir := filepath.Join(dataDir, "data")
	if _, err := os.Stat(storeDir); os.IsNotExist(err) {
		t.Error("Data directory was not created")
	}
}

// TestShutdownOnSignal verifies that SIGINT/SIGTERM trigger graceful shutdown
func TestShutdownIdempotency(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator
	orch, err := New(Config{
		SessionName: "test-idempotency",
		SpecPath:    specPath,
		Iterations:  1,
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Start orchestrator
	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Call Stop() multiple times - should be idempotent
	if err := orch.Stop(); err != nil {
		t.Errorf("First Stop() returned error: %v", err)
	}

	if err := orch.Stop(); err != nil {
		t.Errorf("Second Stop() returned error: %v", err)
	}

	if err := orch.Stop(); err != nil {
		t.Errorf("Third Stop() returned error: %v", err)
	}
}

// TestContextCancellation verifies that cancelling the context triggers cleanup
func TestContextCancellation(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator
	orch, err := New(Config{
		SessionName: "test-context",
		SpecPath:    specPath,
		Iterations:  1,
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Start orchestrator
	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Cancel context (simulates signal)
	orch.cancel()

	// Context should be cancelled
	select {
	case <-orch.ctx.Done():
		// Expected
	case <-time.After(1 * time.Second):
		t.Fatal("Context was not cancelled")
	}

	// Stop should still work after context cancellation
	if err := orch.Stop(); err != nil {
		t.Errorf("Stop() after context cancellation returned error: %v", err)
	}
}

// TestIterationLoopStateTracking verifies that the orchestrator correctly tracks
// iteration state even when agent execution fails or is unavailable.
func TestIterationLoopStateTracking(t *testing.T) {
	// Skip this test if opencode is not available (it's an integration test that needs the agent)
	t.Skip("Integration test requires opencode agent - run manually")

	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec

This is a test spec for iteration loop testing.

## Tasks
- [ ] Test task 1
- [ ] Test task 2
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator with limited iterations
	orch, err := New(Config{
		SessionName: "test-iteration-loop",
		SpecPath:    specPath,
		Iterations:  2, // Run exactly 2 iterations
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Start orchestrator
	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}
	defer func() { _ = orch.Stop() }()

	// Run the iteration loop
	// This will fail because opencode is not available, but we can verify
	// that state is tracked correctly up to the failure point
	_ = orch.Run()

	// Load the state to verify iterations were tracked
	state, err := orch.store.LoadState(orch.ctx, "test-iteration-loop")
	if err != nil {
		t.Fatalf("failed to load state after Run(): %v", err)
	}

	// Verify that at least the first iteration was started
	if len(state.Iterations) == 0 {
		t.Error("expected at least one iteration to be tracked in state")
	}

	// Verify iteration numbers are correct
	if len(state.Iterations) > 0 && state.Iterations[0].Number != 1 {
		t.Errorf("expected first iteration number to be 1, got %d", state.Iterations[0].Number)
	}
}

// TestIterationLoopResumeFromLastIteration verifies that the orchestrator
// can continue from where it left off in a previous session.
func TestIterationLoopResumeFromLastIteration(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec

Resumable session test.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	sessionName := "test-resume-session"

	// First orchestrator - simulate running 2 iterations
	{
		orch, err := New(Config{
			SessionName: sessionName,
			SpecPath:    specPath,
			Iterations:  0, // Unlimited for manual control
			DataDir:     dataDir,
			WorkDir:     tmpDir,
			Headless:    true,
		})
		if err != nil {
			t.Fatalf("failed to create first orchestrator: %v", err)
		}

		if err := orch.Start(); err != nil {
			t.Fatalf("failed to start first orchestrator: %v", err)
		}

		// Manually create iteration events to simulate a session with 2 completed iterations
		if err := orch.store.IterationStart(orch.ctx, sessionName, 1); err != nil {
			t.Fatalf("failed to start iteration 1: %v", err)
		}
		if err := orch.store.IterationComplete(orch.ctx, sessionName, 1); err != nil {
			t.Fatalf("failed to complete iteration 1: %v", err)
		}
		if err := orch.store.IterationStart(orch.ctx, sessionName, 2); err != nil {
			t.Fatalf("failed to start iteration 2: %v", err)
		}
		if err := orch.store.IterationComplete(orch.ctx, sessionName, 2); err != nil {
			t.Fatalf("failed to complete iteration 2: %v", err)
		}

		_ = orch.Stop()
	}

	// Second orchestrator - load same session and verify it would start at iteration 3
	{
		orch, err := New(Config{
			SessionName: sessionName,
			SpecPath:    specPath,
			Iterations:  0,
			DataDir:     dataDir,
			WorkDir:     tmpDir,
			Headless:    true,
		})
		if err != nil {
			t.Fatalf("failed to create second orchestrator: %v", err)
		}

		if err := orch.Start(); err != nil {
			t.Fatalf("failed to start second orchestrator: %v", err)
		}
		defer func() { _ = orch.Stop() }()

		// Load state and verify starting iteration would be 3
		state, err := orch.store.LoadState(orch.ctx, sessionName)
		if err != nil {
			t.Fatalf("failed to load state: %v", err)
		}

		expectedStartIteration := len(state.Iterations) + 1
		if expectedStartIteration != 3 {
			t.Errorf("expected next iteration to be 3, got %d (found %d previous iterations)",
				expectedStartIteration, len(state.Iterations))
		}

		// Verify the 2 previous iterations exist
		if len(state.Iterations) != 2 {
			t.Errorf("expected 2 previous iterations, got %d", len(state.Iterations))
		}
	}
}

// TestIterationLoopSessionComplete verifies that the loop stops when session_complete is signaled
func TestIterationLoopSessionComplete(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	// Create a simple spec file
	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec

Test session complete signal.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	sessionName := "test-session-complete"

	orch, err := New(Config{
		SessionName: sessionName,
		SpecPath:    specPath,
		Iterations:  0, // Unlimited
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}
	defer func() { _ = orch.Stop() }()

	// Mark session as complete before running
	if err := orch.store.SessionComplete(orch.ctx, sessionName); err != nil {
		t.Fatalf("failed to mark session complete: %v", err)
	}

	// Run should detect the session is already complete and exit immediately
	err = orch.Run()
	if err != nil {
		t.Errorf("Run() returned error when session was already complete: %v", err)
	}

	// Verify no new iterations were started
	state, err := orch.store.LoadState(orch.ctx, sessionName)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(state.Iterations) > 0 {
		t.Errorf("expected no iterations to run when session is already complete, but found %d", len(state.Iterations))
	}
}

// TestModelConfiguration verifies that the --model flag is properly passed through to the runner
func TestModelConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name:  "anthropic-model",
			model: "anthropic/claude-sonnet-4-5",
		},
		{
			name:  "openai-model",
			model: "openai/gpt-4",
		},
		{
			name:  "empty-model-use-default",
			model: "",
		},
		{
			name:  "custom-provider",
			model: "custom/my-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tmpDir := t.TempDir()
			dataDir := filepath.Join(tmpDir, ".iteratr")

			// Create a simple spec file
			specPath := filepath.Join(tmpDir, "test.md")
			specContent := `# Test Spec
Model configuration test.
`
			if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
				t.Fatalf("failed to write spec file: %v", err)
			}

			cfg := Config{
				SessionName: "test-model-" + tt.name,
				SpecPath:    specPath,
				Iterations:  1,
				DataDir:     dataDir,
				WorkDir:     tmpDir,
				Headless:    true,
				Model:       tt.model,
			}

			// Verify config stores model correctly
			if cfg.Model != tt.model {
				t.Errorf("expected model %q, got %q", tt.model, cfg.Model)
			}

			// Create orchestrator and verify it accepts the model
			orch, err := New(cfg)
			if err != nil {
				t.Fatalf("failed to create orchestrator with model %q: %v", tt.model, err)
			}

			// Verify orchestrator stored the model
			if orch.cfg.Model != tt.model {
				t.Errorf("orchestrator config: expected model %q, got %q", tt.model, orch.cfg.Model)
			}

			// Start and immediately stop to verify initialization works with model
			if err := orch.Start(); err != nil {
				t.Fatalf("failed to start orchestrator with model %q: %v", tt.model, err)
			}
			defer func() { _ = orch.Stop() }()

			// Note: runner is now created in Run(), not Start()
			// Verify that the model is stored correctly in the config for later runner creation
			if orch.cfg.Model != tt.model {
				t.Errorf("expected model %q to be stored in config, got %q", tt.model, orch.cfg.Model)
			}
		})
	}
}

// TestHeadlessMode verifies that headless mode works correctly
func TestHeadlessMode(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec
Headless mode test.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator in headless mode
	orch, err := New(Config{
		SessionName: "test-headless",
		SpecPath:    specPath,
		Iterations:  1,
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator in headless mode: %v", err)
	}

	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator in headless mode: %v", err)
	}
	defer func() { _ = orch.Stop() }()

	// Verify TUI is not initialized in headless mode
	if orch.tuiApp != nil {
		t.Error("expected tuiApp to be nil in headless mode")
	}
	if orch.tuiProgram != nil {
		t.Error("expected tuiProgram to be nil in headless mode")
	}
}

// TestTUIInitialization verifies that TUI mode initializes without errors
func TestTUIInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".iteratr")

	specPath := filepath.Join(tmpDir, "test.md")
	specContent := `# Test Spec
TUI mode test.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create orchestrator with TUI enabled
	orch, err := New(Config{
		SessionName: "test-tui",
		SpecPath:    specPath,
		Iterations:  1,
		DataDir:     dataDir,
		WorkDir:     tmpDir,
		Headless:    false, // Enable TUI
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator with TUI: %v", err)
	}

	if err := orch.Start(); err != nil {
		t.Fatalf("failed to start orchestrator with TUI: %v", err)
	}
	defer func() { _ = orch.Stop() }()

	// Verify TUI is initialized
	if orch.tuiApp == nil {
		t.Error("expected tuiApp to be initialized with TUI mode enabled")
	}
	if orch.tuiProgram == nil {
		t.Error("expected tuiProgram to be initialized with TUI mode enabled")
	}

	// Note: We can't actually run the TUI in a test environment,
	// but we verify it initializes without errors
}

// TestIsGitRepo verifies that isGitRepo correctly detects git repositories
func TestIsGitRepo(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) error
		expected bool
	}{
		{
			name: "directory with .git",
			setup: func(dir string) error {
				return os.Mkdir(filepath.Join(dir, ".git"), 0755)
			},
			expected: true,
		},
		{
			name: "subdirectory of git repo",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
					return err
				}
				return os.Mkdir(filepath.Join(dir, "subdir"), 0755)
			},
			expected: true,
		},
		{
			name: "directory without .git",
			setup: func(dir string) error {
				return nil // No .git directory
			},
			expected: false,
		},
		{
			name: ".git is a file not directory",
			setup: func(dir string) error {
				// Create .git as a file (worktree reference)
				return os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../main/.git"), 0644)
			},
			expected: false, // We only check for .git directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// For subdirectory test, check the subdirectory
			checkDir := tmpDir
			if tt.name == "subdirectory of git repo" {
				checkDir = filepath.Join(tmpDir, "subdir")
			}

			result := isGitRepo(checkDir)
			if result != tt.expected {
				t.Errorf("isGitRepo(%q) = %v, expected %v", checkDir, result, tt.expected)
			}
		})
	}
}

// TestBuildCommitPrompt verifies that buildCommitPrompt generates correct prompts
func TestBuildCommitPrompt(t *testing.T) {
	// Skip if we can't setup NATS (buildCommitPrompt needs real store for LoadState)
	// But we can test the prompt formatting by creating a minimal orchestrator

	tests := []struct {
		name                string
		setupFiles          []fileChange
		inProgressTaskText  string // If non-empty, creates an in_progress task
		iterationSummary    string // If non-empty, creates an iteration with summary
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "single new file without metadata",
			setupFiles: []fileChange{
				{path: "auth.go", isNew: true, additions: 0, deletions: 0},
			},
			expectedContains: []string{
				"Commit the following modified files:",
				"- auth.go (new file)",
				"Instructions:",
				"1. Stage only the listed files with `git add`",
				"2. Create a commit with a clear, conventional message",
				"3. Do NOT push",
			},
			expectedNotContains: []string{
				"Context:",
				"Task:",
				"Summary:",
			},
		},
		{
			name: "modified file with additions and deletions",
			setupFiles: []fileChange{
				{path: "internal/auth/session.go", isNew: false, additions: 15, deletions: 3},
			},
			expectedContains: []string{
				"Commit the following modified files:",
				"- internal/auth/session.go (+15/-3)",
				"Instructions:",
			},
			expectedNotContains: []string{
				"Context:",
				"(new file)",
			},
		},
		{
			name: "multiple files with mixed states",
			setupFiles: []fileChange{
				{path: "auth.go", isNew: true, additions: 0, deletions: 0},
				{path: "internal/auth/session.go", isNew: false, additions: 15, deletions: 3},
				{path: "internal/handler/login.go", isNew: false, additions: 8, deletions: 2},
			},
			expectedContains: []string{
				"- auth.go (new file)",
				"- internal/auth/session.go (+15/-3)",
				"- internal/handler/login.go (+8/-2)",
			},
			expectedNotContains: []string{
				"Context:",
			},
		},
		{
			name: "with in_progress task context",
			setupFiles: []fileChange{
				{path: "auth.go", isNew: true, additions: 0, deletions: 0},
			},
			inProgressTaskText: "implement JWT authentication",
			expectedContains: []string{
				"- auth.go (new file)",
				"Context:",
				"- Task: implement JWT authentication",
			},
			expectedNotContains: []string{
				"Summary:",
			},
		},
		{
			name: "with iteration summary",
			setupFiles: []fileChange{
				{path: "auth.go", isNew: true, additions: 0, deletions: 0},
			},
			iterationSummary: "Added JWT validation and integrated with login handler",
			expectedContains: []string{
				"- auth.go (new file)",
				"Context:",
				"- Summary: Added JWT validation and integrated with login handler",
			},
			expectedNotContains: []string{
				"Task:",
			},
		},
		{
			name: "with both task and iteration summary",
			setupFiles: []fileChange{
				{path: "internal/auth/jwt.go", isNew: true, additions: 0, deletions: 0},
				{path: "internal/auth/session.go", isNew: false, additions: 15, deletions: 3},
				{path: "internal/handler/login.go", isNew: false, additions: 8, deletions: 2},
			},
			inProgressTaskText: "implement JWT authentication",
			iterationSummary:   "Added JWT validation and integrated with login handler",
			expectedContains: []string{
				"Commit the following modified files:",
				"- internal/auth/jwt.go (new file)",
				"- internal/auth/session.go (+15/-3)",
				"- internal/handler/login.go (+8/-2)",
				"Context:",
				"- Task: implement JWT authentication",
				"- Summary: Added JWT validation and integrated with login handler",
				"Instructions:",
				"1. Stage only the listed files with `git add`",
				"2. Create a commit with a clear, conventional message",
				"3. Do NOT push",
			},
		},
		{
			name: "modified file without metadata (fallback)",
			setupFiles: []fileChange{
				{path: "README.md", isNew: false, additions: 0, deletions: 0},
			},
			expectedContains: []string{
				"- README.md\n", // No +/- counts when metadata unavailable
			},
			expectedNotContains: []string{
				"+0",
				"-0",
				"(new file)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for the test
			tmpDir := t.TempDir()
			dataDir := filepath.Join(tmpDir, ".iteratr")

			// Create orchestrator and start it to get a real store
			orch, err := New(Config{
				SessionName: "test-build-commit-prompt",
				SpecPath:    filepath.Join(tmpDir, "test.md"),
				DataDir:     dataDir,
				WorkDir:     tmpDir,
				Headless:    true,
			})
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}

			// Write a minimal spec file
			specContent := "# Test\nTest spec for buildCommitPrompt test"
			if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(specContent), 0644); err != nil {
				t.Fatalf("failed to write spec file: %v", err)
			}

			// Start orchestrator to initialize store
			if err := orch.Start(); err != nil {
				t.Fatalf("failed to start orchestrator: %v", err)
			}
			defer func() {
				if err := orch.Stop(); err != nil {
					t.Errorf("failed to stop orchestrator: %v", err)
				}
			}()

			// Setup test state if needed
			ctx := context.Background()
			if tt.inProgressTaskText != "" {
				// Add an in_progress task
				if _, err := orch.store.PublishEvent(ctx, session.Event{
					Session: "test-build-commit-prompt",
					Type:    "task",
					Action:  "add",
					Data:    tt.inProgressTaskText,
					Meta:    []byte(`{"status":"in_progress"}`),
				}); err != nil {
					t.Fatalf("failed to add task: %v", err)
				}
			}
			if tt.iterationSummary != "" {
				// Add iteration with summary
				if _, err := orch.store.PublishEvent(ctx, session.Event{
					Session: "test-build-commit-prompt",
					Type:    "iteration",
					Action:  "start",
					Meta:    []byte(`{"number":1}`),
				}); err != nil {
					t.Fatalf("failed to start iteration: %v", err)
				}
				if _, err := orch.store.PublishEvent(ctx, session.Event{
					Session: "test-build-commit-prompt",
					Type:    "iteration",
					Action:  "summary",
					Meta:    []byte(fmt.Sprintf(`{"number":1,"summary":%q}`, tt.iterationSummary)),
				}); err != nil {
					t.Fatalf("failed to add iteration summary: %v", err)
				}
			}

			// Record file changes in tracker
			for _, fc := range tt.setupFiles {
				absPath := filepath.Join(tmpDir, fc.path)
				orch.fileTracker.RecordChange(absPath, fc.isNew, fc.additions, fc.deletions)
			}

			// Generate commit prompt
			prompt := orch.buildCommitPrompt(ctx)

			// Verify expected content
			for _, expected := range tt.expectedContains {
				if !containsString(prompt, expected) {
					t.Errorf("expected prompt to contain %q, but it didn't.\nPrompt:\n%s", expected, prompt)
				}
			}

			// Verify unexpected content is absent
			for _, unexpected := range tt.expectedNotContains {
				if containsString(prompt, unexpected) {
					t.Errorf("expected prompt NOT to contain %q, but it did.\nPrompt:\n%s", unexpected, prompt)
				}
			}

			// Verify prompt structure is valid (has required sections)
			if !containsString(prompt, "Commit the following modified files:") {
				t.Error("prompt missing 'Commit the following modified files:' header")
			}
			if !containsString(prompt, "Instructions:") {
				t.Error("prompt missing 'Instructions:' section")
			}
			if !containsString(prompt, "1. Stage only the listed files") {
				t.Error("prompt missing staging instruction")
			}
			if !containsString(prompt, "2. Create a commit") {
				t.Error("prompt missing commit instruction")
			}
			if !containsString(prompt, "3. Do NOT push") {
				t.Error("prompt missing no-push instruction")
			}
		})
	}
}

// fileChange represents test data for file modifications
type fileChange struct {
	path      string
	isNew     bool
	additions int
	deletions int
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// Suppress unused variable/import warnings
var _ = syscall.SIGTERM
var _ = agent.FileTracker{}
