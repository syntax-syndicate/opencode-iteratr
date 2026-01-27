package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/iteratr/internal/hooks"
	"gopkg.in/yaml.v3"
)

// TestPostIterationIntegration validates the complete flow:
// 1. Load hooks config from file
// 2. Execute post_iteration hooks after iteration completes
// 3. Verify piped output is added to pending buffer
func TestPostIterationIntegration(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create hooks config file
	hooksConfig := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"post_iteration": []map[string]interface{}{
				{
					"command":     "echo 'Test passed'",
					"timeout":     5,
					"pipe_output": true,
				},
				{
					"command":     "echo 'Side effect hook'",
					"timeout":     5,
					"pipe_output": false,
				},
			},
		},
	}

	configPath := filepath.Join(tmpDir, ".iteratr.hooks.yml")
	data, err := yaml.Marshal(hooksConfig)
	if err != nil {
		t.Fatalf("Failed to marshal hooks config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Load hooks config
	loadedConfig, err := hooks.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load hooks config: %v", err)
	}

	if loadedConfig == nil {
		t.Fatal("Expected hooks config to be loaded")
	}

	// Create orchestrator with loaded config
	o := &Orchestrator{
		cfg:         Config{SessionName: "test-session", WorkDir: tmpDir},
		hooksConfig: loadedConfig,
		ctx:         context.Background(),
	}

	// Simulate post_iteration hook execution
	ctx := context.Background()
	hookVars := hooks.Variables{
		Session:   "test-session",
		Iteration: "1",
	}

	output, err := hooks.ExecuteAllPiped(ctx, o.hooksConfig.Hooks.PostIteration, tmpDir, hookVars)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify only piped output is returned
	if output != "Test passed\n" {
		t.Errorf("Expected 'Test passed\\n', got %q", output)
	}

	// Simulate appending to pending buffer
	o.appendPendingOutput(output)

	// Verify pending buffer has the output
	if !o.hasPendingOutput() {
		t.Error("Expected pending output after post_iteration hooks")
	}

	// Drain and verify
	drained := o.drainPendingOutput()
	if drained != "Test passed\n" {
		t.Errorf("Expected 'Test passed\\n', got %q", drained)
	}

	// Buffer should be empty after drain
	if o.hasPendingOutput() {
		t.Error("Expected no pending output after drain")
	}
}
