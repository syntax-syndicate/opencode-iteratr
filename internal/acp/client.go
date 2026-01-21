package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/mark3labs/iteratr/internal/logger"
	"github.com/mark3labs/iteratr/internal/session"
)

// ACPClient implements acp.Client interface for handling agent requests
type ACPClient struct {
	store            *session.Store
	sessionName      string
	workDir          string
	conn             *acp.ClientSideConnection
	currentSessionID acp.SessionId
	sessionComplete  bool
	cmd              *exec.Cmd // Current running subprocess

	// Callback for sending updates to TUI
	onOutput   func(string)
	onToolCall func(string, string) // tool name, status
}

var _ acp.Client = (*ACPClient)(nil)

// NewACPClient creates a new ACP client for managing agent interactions
func NewACPClient(store *session.Store, sessionName, workDir string) *ACPClient {
	return &ACPClient{
		store:       store,
		sessionName: sessionName,
		workDir:     workDir,
	}
}

// SetOutputCallback sets a callback for streaming agent output
func (c *ACPClient) SetOutputCallback(fn func(string)) {
	c.onOutput = fn
}

// SetToolCallCallback sets a callback for tool call updates
func (c *ACPClient) SetToolCallCallback(fn func(string, string)) {
	c.onToolCall = fn
}

// IsSessionComplete returns true if the agent called session_complete
func (c *ACPClient) IsSessionComplete() bool {
	return c.sessionComplete
}

// ReadTextFile implements acp.Client - handles file read requests from agent
func (c *ACPClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	content, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("failed to read file: %w", err)
	}
	return acp.ReadTextFileResponse{Content: string(content)}, nil
}

// WriteTextFile implements acp.Client - handles file write requests from agent
func (c *ACPClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	err := os.WriteFile(params.Path, []byte(params.Content), 0o644)
	if err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("failed to write file: %w", err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// SessionUpdate implements acp.Client - handles streaming updates from agent
func (c *ACPClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update

	switch {
	case u.AgentMessageChunk != nil:
		// Stream agent text output to TUI
		if u.AgentMessageChunk.Content.Text != nil && c.onOutput != nil {
			c.onOutput(u.AgentMessageChunk.Content.Text.Text)
		}

	case u.ToolCall != nil:
		// Tool call initiated - check if it's one of our custom tools
		return c.handleToolCall(ctx, u.ToolCall)

	case u.ToolCallUpdate != nil:
		// Tool call completed
		if c.onToolCall != nil {
			c.onToolCall(string(u.ToolCallUpdate.ToolCallId), string(*u.ToolCallUpdate.Status))
		}
	}

	return nil
}

// RequestPermission implements acp.Client - handles permission requests
func (c *ACPClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// For now, auto-approve all requests
	// In the future, this could prompt the user via TUI
	if len(params.Options) == 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Cancelled: &acp.RequestPermissionOutcomeCancelled{
					Outcome: "cancelled",
				},
			},
		}, nil
	}

	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{
				OptionId: params.Options[0].OptionId,
				Outcome:  "selected",
			},
		},
	}, nil
}

// CreateTerminal implements acp.Client - creates a terminal for command execution
func (c *ACPClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	// For now, we don't support terminal creation
	return acp.CreateTerminalResponse{}, fmt.Errorf("terminal support not implemented")
}

// KillTerminalCommand implements acp.Client - kills a running terminal command
func (c *ACPClient) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, fmt.Errorf("terminal support not implemented")
}

// TerminalOutput implements acp.Client - gets terminal output and status
func (c *ACPClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("terminal support not implemented")
}

// ReleaseTerminal implements acp.Client - releases a terminal and frees resources
func (c *ACPClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("terminal support not implemented")
}

// WaitForTerminalExit implements acp.Client - waits for terminal command to exit
func (c *ACPClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("terminal support not implemented")
}

// handleToolCall processes tool calls from the agent and routes them to the session store
// In ACP, the client receives tool calls via SessionUpdate and processes them inline.
// The agent is responsible for managing tool state; we just execute and return errors if any.
func (c *ACPClient) handleToolCall(ctx context.Context, tc *acp.SessionUpdateToolCall) error {
	// Extract tool name from Title or RawInput
	// The agent encodes the tool name in the call
	toolName := tc.Title
	input, ok := tc.RawInput.(map[string]any)
	if !ok {
		input = make(map[string]any)
	}

	var result any
	var err error

	// Route to appropriate session store method based on tool name
	switch toolName {
	case "task_add":
		result, err = c.handleTaskAdd(ctx, input)
	case "task_status":
		result, err = c.handleTaskStatus(ctx, input)
	case "task_list":
		result, err = c.handleTaskList(ctx)
	case "note_add":
		result, err = c.handleNoteAdd(ctx, input)
	case "note_list":
		result, err = c.handleNoteList(ctx, input)
	case "inbox_list":
		result, err = c.handleInboxList(ctx)
	case "inbox_mark_read":
		result, err = c.handleInboxMarkRead(ctx, input)
	case "session_complete":
		result, err = c.handleSessionComplete(ctx)
		c.sessionComplete = true
	default:
		// Not our tool, agent will handle it
		return nil
	}

	// If there's an error, report it
	if err != nil {
		if c.onToolCall != nil {
			c.onToolCall(toolName, "failed: "+err.Error())
		}
		return err
	}

	// Notify success
	if c.onToolCall != nil {
		c.onToolCall(toolName, formatResult(result, nil))
	}

	return nil
}

// formatResult formats a tool result for display to the agent
func formatResult(result any, err error) string {
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if result == nil {
		return "OK"
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b)
}

// RunIteration launches the agent subprocess and runs a single iteration
func (c *ACPClient) RunIteration(ctx context.Context, prompt string) error {
	logger.Debug("Starting agent iteration")

	// Validate inputs
	if prompt == "" {
		logger.Error("Prompt is empty")
		return fmt.Errorf("prompt cannot be empty")
	}
	if c.workDir == "" {
		logger.Error("Working directory not set")
		return fmt.Errorf("working directory not set")
	}

	// Start opencode as subprocess
	logger.Debug("Launching opencode subprocess")
	cmd := exec.CommandContext(ctx, "opencode", "acp")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Error("Failed to create stdin pipe: %v", err)
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("Failed to create stdout pipe: %v", err)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start opencode: %v", err)
		return fmt.Errorf("failed to start opencode: %w", err)
	}
	logger.Debug("opencode subprocess started")

	// Store command reference for cleanup
	c.cmd = cmd

	// Create client-side connection
	logger.Debug("Creating ACP connection")
	c.conn = acp.NewClientSideConnection(c, stdin, stdout)

	// Initialize connection
	logger.Debug("Initializing ACP connection")
	_, err = c.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
		},
	})
	if err != nil {
		logger.Error("ACP initialize failed: %v", err)
		return fmt.Errorf("initialize failed: %w", err)
	}
	logger.Debug("ACP connection initialized")

	// Create session
	logger.Debug("Creating ACP session")
	newSess, err := c.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd: c.workDir,
	})
	if err != nil {
		logger.Error("ACP new session failed: %v", err)
		return fmt.Errorf("new session failed: %w", err)
	}
	logger.Debug("ACP session created: %s", newSess.SessionId)

	c.currentSessionID = newSess.SessionId

	// Send prompt and stream response
	logger.Debug("Sending prompt to agent (length: %d chars)", len(prompt))
	_, err = c.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: newSess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		logger.Error("ACP prompt failed: %v", err)
		return fmt.Errorf("prompt failed: %w", err)
	}
	logger.Debug("Prompt sent, waiting for agent to complete")

	// Wait for agent process to complete
	if err := cmd.Wait(); err != nil {
		logger.Error("Agent process failed: %v", err)
		return fmt.Errorf("agent process failed: %w", err)
	}
	logger.Debug("Agent process completed successfully")

	// Clear command reference after completion
	c.cmd = nil

	return nil
}

// Cleanup terminates any running agent subprocess
func (c *ACPClient) Cleanup() error {
	if c.cmd != nil && c.cmd.Process != nil {
		// Try graceful termination first
		if err := c.cmd.Process.Signal(os.Interrupt); err == nil {
			// Wait for process to exit with timeout
			done := make(chan error, 1)
			go func() {
				done <- c.cmd.Wait()
			}()

			select {
			case <-done:
				// Process exited
			case <-time.After(2 * time.Second):
				// Timeout - force kill
				if err := c.cmd.Process.Kill(); err != nil {
					return fmt.Errorf("failed to kill agent process: %w", err)
				}
			}
		}
		c.cmd = nil
	}
	return nil
}
