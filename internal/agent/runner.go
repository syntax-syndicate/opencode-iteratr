package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mark3labs/iteratr/internal/logger"
)

// Runner manages the execution of opencode run subprocess for each iteration.
type Runner struct {
	model       string
	workDir     string
	sessionName string
	natsPort    int
	onText      func(text string)
	onToolCall  func(ToolCallEvent)
	onThinking  func(string)
	onFinish    func(FinishEvent)

	// Persistent ACP session fields
	conn      *acpConn
	sessionID string
	cmd       *exec.Cmd
}

// RunnerConfig holds configuration for creating a new Runner.
type RunnerConfig struct {
	Model       string              // LLM model to use (e.g., "anthropic/claude-sonnet-4-5")
	WorkDir     string              // Working directory for agent
	SessionName string              // Session name
	NATSPort    int                 // NATS server port for tool CLI
	OnText      func(text string)   // Callback for text output
	OnToolCall  func(ToolCallEvent) // Callback for tool lifecycle events
	OnThinking  func(string)        // Callback for thinking/reasoning output
	OnFinish    func(FinishEvent)   // Callback for iteration finish events
}

// NewRunner creates a new Runner instance.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{
		model:       cfg.Model,
		workDir:     cfg.WorkDir,
		sessionName: cfg.SessionName,
		natsPort:    cfg.NATSPort,
		onText:      cfg.OnText,
		onToolCall:  cfg.OnToolCall,
		onThinking:  cfg.OnThinking,
		onFinish:    cfg.OnFinish,
	}
}

// extractProvider parses provider name from model string.
// Model format is typically "provider/model-name" (e.g., "anthropic/claude-sonnet-4-5").
// Returns capitalized provider name (e.g., "Anthropic") or empty string if no slash.
func extractProvider(model string) string {
	if idx := strings.Index(model, "/"); idx >= 0 {
		provider := model[:idx]
		// Capitalize first letter
		if len(provider) > 0 {
			return strings.ToUpper(provider[:1]) + provider[1:]
		}
		return provider
	}
	return ""
}

// Start initializes the persistent ACP session by spawning opencode acp subprocess
// and performing the initialize → newSession → setModel sequence.
// Must be called before RunIteration.
func (r *Runner) Start(ctx context.Context) error {
	logger.Debug("Starting persistent ACP session")

	// Create command - spawn opencode acp
	cmd := exec.CommandContext(ctx, "opencode", "acp")
	cmd.Dir = r.workDir
	cmd.Env = os.Environ()
	// Don't inherit stderr - it corrupts terminal state during TUI shutdown
	// Subprocess errors are captured via the ACP protocol

	// Setup stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Setup stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	logger.Debug("Starting opencode subprocess")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start opencode: %w", err)
	}

	// Create acpConn from stdin/stdout pipes
	conn := newACPConn(stdin, stdout)

	// Call initialize → newSession → setModel sequence
	if err := conn.initialize(ctx); err != nil {
		_ = conn.close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("ACP initialize failed: %w", err)
	}

	sessID, err := conn.newSession(ctx, r.workDir)
	if err != nil {
		_ = conn.close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("ACP new session failed: %w", err)
	}

	// Set model if configured
	if r.model != "" {
		logger.Debug("Setting model: %s", r.model)
		if err := conn.setModel(ctx, sessID, r.model); err != nil {
			_ = conn.close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return fmt.Errorf("ACP set model failed: %w", err)
		}
	}

	// Store persistent session state
	r.conn = conn
	r.sessionID = sessID
	r.cmd = cmd

	logger.Debug("Persistent ACP session ready: sessionID=%s", sessID)
	return nil
}

// RunIteration executes a single iteration by sending a prompt to the persistent ACP session.
// Optional hookOutput is sent as a separate content block before the main prompt.
// Start() must be called first to initialize the session.
func (r *Runner) RunIteration(ctx context.Context, prompt string, hookOutput string) error {
	if r.conn == nil {
		return fmt.Errorf("ACP session not started - call Start() first")
	}

	logger.Debug("Running iteration on existing ACP session")

	// Build content blocks: hook output (if any) + main prompt
	var texts []string
	if hookOutput != "" {
		texts = append(texts, hookOutput)
		logger.Debug("Including hook output: %d bytes", len(hookOutput))
	}
	texts = append(texts, prompt)

	// Send prompt and stream notifications to callbacks
	// Wire onText, onToolCall, and onThinking callbacks through to prompt()
	startTime := time.Now()
	stopReason, err := r.conn.prompt(ctx, r.sessionID, texts, r.onText, r.onToolCall, r.onThinking)
	duration := time.Since(startTime)

	if err != nil {
		// Prompt failed - determine if it was cancelled or error
		if r.onFinish != nil {
			finalStopReason := "error"
			if ctx.Err() == context.Canceled {
				finalStopReason = "cancelled"
			}
			r.onFinish(FinishEvent{
				StopReason: finalStopReason,
				Error:      err.Error(),
				Duration:   duration,
				Model:      r.model,
				Provider:   extractProvider(r.model),
			})
		}
		return fmt.Errorf("ACP prompt failed: %w", err)
	}

	// Prompt succeeded - call onFinish with the actual stop reason from ACP
	if r.onFinish != nil {
		r.onFinish(FinishEvent{
			StopReason: stopReason,
			Duration:   duration,
			Model:      r.model,
			Provider:   extractProvider(r.model),
		})
	}

	logger.Debug("opencode iteration completed successfully")
	return nil
}

// SendMessages sends multiple user messages to the persistent ACP session as a single prompt.
// Each message becomes a separate content block in the request.
// This allows batching queued user input while keeping them logically distinct.
// Start() must be called first to initialize the session.
func (r *Runner) SendMessages(ctx context.Context, texts []string) error {
	if r.conn == nil {
		return fmt.Errorf("ACP session not started - call Start() first")
	}

	if len(texts) == 0 {
		return nil
	}

	logger.Debug("Sending %d user message(s) to ACP session", len(texts))

	// Send prompt with all messages as separate content blocks
	startTime := time.Now()
	stopReason, err := r.conn.prompt(ctx, r.sessionID, texts, r.onText, r.onToolCall, r.onThinking)
	duration := time.Since(startTime)

	if err != nil {
		// Prompt failed - determine if it was cancelled or error
		if r.onFinish != nil {
			finalStopReason := "error"
			if ctx.Err() == context.Canceled {
				finalStopReason = "cancelled"
			}
			r.onFinish(FinishEvent{
				StopReason: finalStopReason,
				Error:      err.Error(),
				Duration:   duration,
				Model:      r.model,
				Provider:   extractProvider(r.model),
			})
		}
		return fmt.Errorf("ACP user message failed: %w", err)
	}

	// Prompt succeeded - call onFinish with the actual stop reason from ACP
	if r.onFinish != nil {
		r.onFinish(FinishEvent{
			StopReason: stopReason,
			Duration:   duration,
			Model:      r.model,
			Provider:   extractProvider(r.model),
		})
	}

	logger.Debug("User message processed successfully")
	return nil
}

// Stop terminates the persistent ACP session and cleans up resources.
// Should be called when done with the runner (e.g., on orchestrator exit).
func (r *Runner) Stop() {
	if r.conn != nil {
		logger.Debug("Closing ACP connection")
		_ = r.conn.close()
		r.conn = nil
	}
	if r.cmd != nil && r.cmd.Process != nil {
		logger.Debug("Terminating opencode subprocess")
		_ = r.cmd.Process.Kill()
		_ = r.cmd.Wait()
		r.cmd = nil
	}
	r.sessionID = ""
	logger.Debug("ACP session stopped")
}
