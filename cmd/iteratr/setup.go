package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/iteratr/internal/config"
	"github.com/spf13/cobra"
)

var setupFlags struct {
	project bool
	force   bool
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create iteratr configuration file",
	Long: `Create an iteratr configuration file with sensible defaults.

By default, creates a global config at ~/.config/iteratr/iteratr.yml.
Use --project to create a project-local config in the current directory.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVarP(&setupFlags.project, "project", "p", false, "Create config in current directory instead of global location")
	setupCmd.Flags().BoolVarP(&setupFlags.force, "force", "f", false, "Overwrite existing config file")
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Determine target path
	targetPath := config.GlobalPath()
	if setupFlags.project {
		targetPath = config.ProjectPath()
	}

	// Check if config already exists
	if !setupFlags.force && fileExists(targetPath) {
		return fmt.Errorf("config file already exists at %s\n\nUse --force to overwrite", targetPath)
	}

	// Create hardcoded config for tracer bullet phase
	// TODO: Replace with TUI wizard in later tasks
	cfg := &config.Config{
		Model:      "anthropic/claude-sonnet-4-5", // Hardcoded for tracer bullet
		AutoCommit: true,
		DataDir:    ".iteratr",
		LogLevel:   "info",
		LogFile:    "",
		Iterations: 0,
		Headless:   false,
		Template:   "",
	}

	// Write config to target location
	var err error
	if setupFlags.project {
		err = config.WriteProject(cfg)
	} else {
		err = config.WriteGlobal(cfg)
	}

	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Print success message
	fmt.Printf("Config written to: %s\n\n", targetPath)
	fmt.Println("Run 'iteratr build' to get started.")

	return nil
}

// fileExists checks if a file exists (helper for setup command).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
