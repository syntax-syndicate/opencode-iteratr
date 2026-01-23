# ACP Migration

Migrate from `opencode run --format json` back to ACP protocol (`opencode acp`) for agent communication.

## Overview

Replace managed process JSON event parsing with ACP (Agent Control Protocol) over stdio. Model selection moves from `--model` CLI flag to `session/set_model` JSON-RPC call. Tools remain as CLI subcommands called by agent via Bash (unchanged). Streaming output received via `session/update` notifications instead of JSON event lines.

## User Story

**As a** developer using iteratr  
**I want** ACP-based communication with opencode  
**So that** I get a proper bidirectional protocol with structured session management, model switching, and streaming notifications

## Requirements

### Functional

1. **ACP Connection Lifecycle**
   - Spawn `opencode acp` subprocess per iteration
   - Establish bidirectional JSON-RPC 2.0 over stdin/stdout
   - `initialize` → `session/new` → `session/set_model` → `session/prompt` flow
   - Graceful shutdown: close connection, then kill process

2. **Model Selection via ACP**
   - After `session/new`, call `session/set_model` with configured model ID
   - Model IDs match opencode format: `provider/model` (e.g., `anthropic/claude-sonnet-4-0`)
   - If no model specified, use opencode's default (from `session/new` response)
   - Handle error if model not available

3. **Streaming via SessionUpdate Notifications**
   - Receive `session/update` notifications (no request ID, one-way from server)
   - Handle `agent_message_chunk` → forward text to TUI/stdout
   - Handle `tool_call` → forward to TUI (tool started, pending)
   - Handle `tool_call_update` (in_progress) → forward to TUI (shows command being run)
   - Handle `tool_call_update` (completed) → forward to TUI (shows output)
   - Handle `available_commands_update` → ignore (informational)

4. **Tool Invocation (Unchanged)**
   - Agent calls `iteratr tool ...` via Bash (existing mechanism)
   - Prompt template keeps `{{binary}}` and `{{port}}` placeholders
   - NATS TCP listener unchanged

5. **Prompt Delivery**
   - Send prompt via `session/prompt` request (not stdin pipe)
   - Prompt wrapped in `ContentBlock` array with text type
   - Wait for response (completion or error)

6. **TUI Real-Time Tool Display**
   - Show tool calls inline in agent output with lifecycle status
   - `tool_call` (pending): show tool name with spinner indicator
   - `tool_call_update` (in_progress): show command/params being executed
   - `tool_call_update` (completed): show output, mark done with checkmark
   - `tool_call_update` (completed) with error: show error in red
   - Track active tool calls by `toolCallId` for in-place updates
   - Tool output collapsed by default if long (>3 lines), expandable

### Non-Functional

1. Maintain session data backwards compatibility (NATS unchanged)
2. `--model` CLI flag still works, value passed to `session/set_model`
3. Graceful fallback if ACP initialization fails
4. Context cancellation kills subprocess

## Technical Implementation

### Architecture Change

**Before (Managed Process):**
```
iteratr ──stdin(prompt)/stdout(json)──> opencode run --format json --model X
                                            │
                                            └──Bash──> iteratr tool <cmd>
```

**After (ACP):**
```
iteratr ──JSON-RPC/stdio──> opencode acp
    │                            │
    │   (session/update notifs)  │
    │<───────────────────────────│
    │                            │
    │                            └──Bash──> iteratr tool <cmd>
```

### ACP Protocol Flow (Verified Working)

```
Client (iteratr)                          Server (opencode acp)
    │                                          │
    │──initialize(protocolVersion:1)──────────>│
    │<─────────────────────result(agentInfo)───│
    │                                          │
    │──session/new(cwd, mcpServers:[])────────>│
    │<───────result(sessionId, models, modes)──│
    │<───────session/update(available_commands)─│
    │                                          │
    │──session/set_model(sessionId, modelId)──>│
    │<─────────────────────result({_meta:{}})──│
    │                                          │
    │──session/prompt(sessionId, prompt)──────>│
    │<───────session/update(AgentMessageChunk)─│  (repeated)
    │<───────session/update(ToolCall)──────────│  (agent uses bash)
    │<───────session/update(ToolCallUpdate)────│
    │<───────session/update(AgentMessageChunk)─│
    │<─────────────────────result(completion)──│
    │                                          │
    │  [close stdin / kill process]            │
```

### Key Protocol Details (From Testing)

- `protocolVersion` must be numeric `1` (not string)
- `session/new` requires `mcpServers: []` (empty array, not omitted)
- `session/set_model` is the wire method name (Go SDK constant: `AgentMethodSessionSetModel`)
- Notifications have `method` field but no `id` (one-way)
- Prompt response arrives after all `session/update` notifications complete
- Tool calls produce 5-6 notifications: 1 `tool_call` (pending) + 3-4 `tool_call_update` (in_progress) + 1 `tool_call_update` (completed)
- Text chunks are small (word/phrase level), arrive after tool calls complete

### Verified Wire Format Examples

**agent_message_chunk:**
```json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_...","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello"}}}}
```

**tool_call (pending):**
```json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_...","update":{"sessionUpdate":"tool_call","toolCallId":"toolu_01Ww5...","title":"bash","kind":"execute","status":"pending","locations":[],"rawInput":{}}}}
```

**tool_call_update (in_progress, with params):**
```json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_...","update":{"sessionUpdate":"tool_call_update","toolCallId":"toolu_01Ww5...","status":"in_progress","kind":"execute","title":"bash","locations":[],"rawInput":{"command":"echo hello-from-acp","description":"Echo the string"}}}}
```

**tool_call_update (completed, with output):**
```json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_...","update":{"sessionUpdate":"tool_call_update","toolCallId":"toolu_01Ww5...","status":"completed","kind":"execute","content":[{"type":"content","content":{"type":"text","text":"hello-from-acp\n"}}],"title":"Echo the string","rawInput":{"command":"echo hello-from-acp","description":"Echo the string"},"rawOutput":{"output":"hello-from-acp\n"}}}}
```

**prompt result:**
```json
{"jsonrpc":"2.0","id":4,"result":{"stopReason":"end_turn","_meta":{}}}
```

### Package Changes

```
internal/
  agent/
    runner.go      # REWRITE - ACP connection instead of JSON parsing
    acp.go         # NEW - ACP protocol helpers (send/receive, types)
  template/
    default.go     # UNCHANGED - tools still Bash commands
    template.go    # UNCHANGED
  orchestrator/
    orchestrator.go # MINOR - remove OnToolUse/OnError callbacks, simplify runner creation
```

### ACP Message Types

```go
// internal/agent/acp.go

// JSON-RPC 2.0 message envelope
type jsonRPCRequest struct {
    JSONRPC string `json:"jsonrpc"`
    ID      int    `json:"id"`
    Method  string `json:"method"`
    Params  any    `json:"params"`
}

type jsonRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      *int            `json:"id,omitempty"`      // nil for notifications
    Method  string          `json:"method,omitempty"`  // set for notifications
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *jsonRPCError   `json:"error,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`  // for notifications
}

type jsonRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// ACP-specific types
type initializeParams struct {
    ProtocolVersion    int                `json:"protocolVersion"`
    ClientCapabilities clientCapabilities `json:"clientCapabilities"`
}

type clientCapabilities struct {
    Fs fsCapability `json:"fs"`
}

type fsCapability struct {
    ReadTextFile  bool `json:"readTextFile"`
    WriteTextFile bool `json:"writeTextFile"`
}

type newSessionParams struct {
    Cwd        string `json:"cwd"`
    McpServers []any  `json:"mcpServers"`
}

type newSessionResult struct {
    SessionID string       `json:"sessionId"`
    Models    *modelsState `json:"models,omitempty"`
}

type modelsState struct {
    CurrentModelID  string      `json:"currentModelId"`
    AvailableModels []modelInfo `json:"availableModels"`
}

type modelInfo struct {
    ModelID string `json:"modelId"`
    Name    string `json:"name"`
}

type setModelParams struct {
    SessionID string `json:"sessionId"`
    ModelID   string `json:"modelId"`
}

type promptParams struct {
    SessionID string         `json:"sessionId"`
    Prompt    []contentBlock `json:"prompt"`
}

type contentBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

// SessionUpdate notification params
type sessionUpdateParams struct {
    SessionID string          `json:"sessionId"`
    Update    json.RawMessage `json:"update"`
}

// Discriminated by "sessionUpdate" field
type sessionUpdate struct {
    SessionUpdate string `json:"sessionUpdate"` // "agent_message_chunk", "tool_call", "tool_call_update", "available_commands_update"
}

// agent_message_chunk
type agentMessageChunk struct {
    SessionUpdate string      `json:"sessionUpdate"` // "agent_message_chunk"
    Content       contentPart `json:"content"`
}

type contentPart struct {
    Type string `json:"type"` // "text"
    Text string `json:"text"`
}

// tool_call (initial, status=pending)
type toolCall struct {
    SessionUpdate string         `json:"sessionUpdate"` // "tool_call"
    ToolCallID    string         `json:"toolCallId"`
    Title         string         `json:"title"`         // tool name, e.g. "bash"
    Kind          string         `json:"kind"`          // "execute"
    Status        string         `json:"status"`        // "pending"
    RawInput      map[string]any `json:"rawInput"`      // empty on pending
}

// tool_call_update (in_progress or completed)
type toolCallUpdate struct {
    SessionUpdate string              `json:"sessionUpdate"` // "tool_call_update"
    ToolCallID    string              `json:"toolCallId"`
    Title         string              `json:"title"`
    Kind          string              `json:"kind"`
    Status        string              `json:"status"`        // "in_progress" | "completed"
    RawInput      map[string]any      `json:"rawInput"`      // e.g. {"command":"echo hi","description":"..."}
    Content       []toolCallContent   `json:"content,omitempty"` // only on completed
    RawOutput     map[string]any      `json:"rawOutput,omitempty"` // only on completed
}

// Nested content in completed tool_call_update
type toolCallContent struct {
    Type    string      `json:"type"`    // "content"
    Content contentPart `json:"content"` // {type:"text", text:"output here"}
}
```

### Runner Rewrite

```go
// internal/agent/runner.go - core changes

func (r *Runner) RunIteration(ctx context.Context, prompt string) error {
    cmd := exec.CommandContext(ctx, "opencode", "acp")
    cmd.Dir = r.workDir
    cmd.Env = os.Environ()
    cmd.Stderr = os.Stderr

    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    conn := newACPConn(stdin, stdout)
    defer func() {
        conn.close()
        cmd.Process.Kill()
        cmd.Wait()
    }()

    // Initialize
    if err := conn.initialize(ctx); err != nil {
        return fmt.Errorf("ACP initialize failed: %w", err)
    }

    // Create session
    sessID, err := conn.newSession(ctx, r.workDir)
    if err != nil {
        return fmt.Errorf("ACP new session failed: %w", err)
    }

    // Set model if configured
    if r.model != "" {
        if err := conn.setModel(ctx, sessID, r.model); err != nil {
            return fmt.Errorf("ACP set model failed: %w", err)
        }
    }

    // Send prompt and stream notifications to callbacks
    return conn.prompt(ctx, sessID, prompt, r.onText, r.onToolCall)
}
```

### Callbacks

```go
// Runner callbacks - wired by orchestrator
type RunnerConfig struct {
    Model       string
    WorkDir     string
    SessionName string
    NATSPort    int
    OnText      func(text string)                          // agent_message_chunk → append text
    OnToolCall  func(event ToolCallEvent)                  // tool_call + tool_call_update → TUI
}

// ToolCallEvent represents a tool lifecycle event from ACP
type ToolCallEvent struct {
    ToolCallID string            // Stable ID for tracking updates
    Title      string            // Tool name (e.g., "bash")
    Status     string            // "pending", "in_progress", "completed"
    RawInput   map[string]any    // Command params (populated on in_progress+)
    Output     string            // Tool output (populated on completed)
    Kind       string            // "execute", etc.
}
```

### TUI Message Types

```go
// Replace existing AgentToolMsg with richer lifecycle messages

// AgentToolCallMsg - tool call started or updated
type AgentToolCallMsg struct {
    ToolCallID string         // Stable ID for in-place updates
    Title      string         // Tool name
    Status     string         // "pending", "in_progress", "completed"
    Input      map[string]any // Raw input params (command, description, etc.)
    Output     string         // Result text (only on completed)
}
```

### Agent Output Component Changes

The agent output component tracks active tool calls by ID and updates them in-place:

```go
type AgentOutput struct {
    viewport    viewport.Model
    messages    []AgentMessage
    toolIndex   map[string]int  // toolCallId → index in messages slice
    // ...existing fields
}

// AppendToolCall handles all tool lifecycle events
func (a *AgentOutput) AppendToolCall(msg AgentToolCallMsg) tea.Cmd {
    idx, exists := a.toolIndex[msg.ToolCallID]
    if !exists {
        // New tool call - append message
        a.messages = append(a.messages, AgentMessage{
            Type:       MessageTypeTool,
            Tool:       msg.Title,
            ToolStatus: msg.Status,
            Content:    formatToolInput(msg.Input),
        })
        a.toolIndex[msg.ToolCallID] = len(a.messages) - 1
    } else {
        // Update existing tool call in-place
        m := &a.messages[idx]
        m.ToolStatus = msg.Status
        if len(msg.Input) > 0 {
            m.Content = formatToolInput(msg.Input)
        }
        if msg.Output != "" {
            m.ToolOutput = msg.Output
        }
    }
    a.refreshContent()
    return nil
}
```

### Tool Rendering (Enhanced)

```
Pending:
┃ ⠋ bash
│

In-progress:
┃ ⠋ bash
│ command: echo hello-from-acp
│

Completed (short output):
┃ ✓ bash
│ command: echo hello-from-acp
│ ─── output ───
│ hello-from-acp
│

Completed (long output, collapsed):
┃ ✓ bash
│ command: echo hello-from-acp
│ ─── output (12 lines) ───
│ first line...
│ [collapsed]
```

### go.mod

No new dependencies needed. ACP is implemented as raw JSON-RPC over stdio (no SDK import required). The protocol is simple enough to implement directly with `encoding/json` and `bufio`.

**Rationale**: The `acp-go-sdk` is designed for implementing ACP agents (server-side). We're a client. The wire protocol is just JSON-RPC 2.0 - 4 request types and 1 notification type. Implementing directly avoids a dependency and gives us full control.

## Tasks

### 1. Create ACP Connection Helper
- [ ] Create `internal/agent/acp.go` with `acpConn` struct wrapping stdin/stdout
- [ ] Implement `newACPConn(stdin io.WriteCloser, stdout io.Reader) *acpConn`
- [ ] Implement `sendRequest(method string, params any) (int, error)` - write JSON-RPC request, return assigned ID
- [ ] Implement `readMessage() (*jsonRPCResponse, error)` - read one JSON line from stdout
- [ ] Implement `close()` - close stdin to signal EOF to subprocess
- [ ] Add atomic request ID counter

### 2. Implement ACP Protocol Methods
- [ ] Implement `initialize(ctx) error` - send initialize, read result, validate agentInfo
- [ ] Implement `newSession(ctx, cwd string) (sessionID string, err error)` - send session/new, parse sessionId from result
- [ ] Implement `setModel(ctx, sessionID, modelID string) error` - send session/set_model, check for error response

### 3. Implement prompt() with Notification Streaming
- [ ] Implement `prompt(ctx, sessionID, text string, onText func(string), onToolCall func(ToolCallEvent)) error`
- [ ] Read messages in loop until response with matching request ID arrives
- [ ] For notifications (id==nil, method=="session/update"): parse update params
- [ ] `agent_message_chunk`: extract `update.content.text`, call `onText`
- [ ] `tool_call`: build `ToolCallEvent{Status:"pending", ToolCallID, Title, Kind}`, call `onToolCall`
- [ ] `tool_call_update` (in_progress): build event with `RawInput` from update, call `onToolCall`
- [ ] `tool_call_update` (completed): build event with `Output` from `update.content[0].content.text`, call `onToolCall`
- [ ] `available_commands_update`: skip
- [ ] Return nil on successful prompt result, wrap error on error result

### 4. Add ToolCallEvent Type and Update Runner
- [ ] Define `ToolCallEvent` struct in `internal/agent/types.go` (ToolCallID, Title, Status, RawInput, Output, Kind)
- [ ] Add `OnToolCall func(ToolCallEvent)` to `RunnerConfig` and `Runner`
- [ ] Remove `OnToolUse` and `OnError` from `RunnerConfig` (replaced by OnToolCall, errors returned directly)
- [ ] Keep `OnText` callback

### 5. Rewrite Runner.RunIteration to Use ACP
- [ ] Replace body: spawn `opencode acp` instead of `opencode run --format json`
- [ ] Remove `--model` flag from command args
- [ ] Create `acpConn` from stdin/stdout pipes
- [ ] Call `initialize` → `newSession` → `setModel` → `prompt` sequence
- [ ] Wire both `onText` and `onToolCall` callbacks through to `prompt()`
- [ ] Keep context cancellation / process kill cleanup logic
- [ ] Delete `parseEvent` method (no longer needed)

### 6. Update Orchestrator Runner Creation
- [ ] Replace `OnToolUse` callback with `OnToolCall` in orchestrator.go
- [ ] TUI path: send `tui.AgentToolCallMsg` in OnToolCall callback
- [ ] Headless path: print tool status to stdout (e.g., `[tool: bash] echo hello...`)
- [ ] Remove `OnError` callback (errors returned from RunIteration directly)
- [ ] Keep `OnText` callback for both TUI and headless modes

### 7. Add TUI AgentToolCallMsg Message Type
- [ ] Replace `AgentToolMsg` with `AgentToolCallMsg` in app.go message types
- [ ] Fields: ToolCallID, Title, Status, Input map[string]any, Output string
- [ ] Update app.go `Update()` to route `AgentToolCallMsg` to `a.agent.AppendToolCall(msg)`
- [ ] Remove old `AgentToolMsg` handling

### 8. Add toolIndex to AgentOutput Component
- [ ] Add `toolIndex map[string]int` field to `AgentOutput` struct (toolCallId → message index)
- [ ] Add `ToolStatus string` and `ToolOutput string` fields to `AgentMessage` struct
- [ ] Initialize `toolIndex` in constructor (`NewAgentOutput` or wherever)
- [ ] Implement `AppendToolCall(msg AgentToolCallMsg) tea.Cmd`:
  - If toolCallId not in toolIndex: append new message, store index
  - If toolCallId exists: update message in-place (status, input, output)
  - Call `refreshContent()`
- [ ] Remove old `AppendTool` method

### 9. Enhanced Tool Rendering in AgentOutput
- [ ] Update `renderToolMessage` to show lifecycle status:
  - Pending: spinner icon + tool name (dim)
  - In-progress: spinner icon + tool name + command params
  - Completed: checkmark + tool name + command + output
- [ ] Use `colorSuccess` for completed checkmark, `colorWarning` for spinner, `colorError` for failed
- [ ] Show tool output below command (indented, muted color)
- [ ] If output > 3 lines: show first 3 lines + "(N more lines)" suffix
- [ ] Tool border: left thick border in `colorSecondary` (existing style)

### 10. Headless Mode Tool Display
- [ ] In headless `OnToolCall` callback: print tool lifecycle to stdout
- [ ] Pending: `[tool: bash] ...`
- [ ] In-progress: `[tool: bash] command: echo hello`
- [ ] Completed: `[tool: bash] ✓ (output: N lines)`
- [ ] Keep output brief - don't dump full tool output in headless

### 11. Testing
- [ ] Manual test: run `iteratr build --model anthropic/claude-haiku-4-5` with ACP, verify model set
- [ ] Manual test: verify agent text streams to TUI in real-time
- [ ] Manual test: verify tool calls show in TUI with pending → in_progress → completed lifecycle
- [ ] Manual test: verify tool output displays correctly (short and long)
- [ ] Manual test: verify agent can call `iteratr tool` commands via Bash
- [ ] Manual test: verify session_complete still ends the loop
- [ ] Manual test: verify headless mode prints text + tool status
- [ ] Manual test: verify Ctrl+C gracefully kills opencode subprocess

## Out of Scope

- Using `acp-go-sdk` (we implement raw JSON-RPC - simpler, no dependency)
- Changing tool invocation to ACP native tool calls (tools stay as Bash CLI)
- Handling `ReadTextFile`/`WriteTextFile` requests from opencode (opencode handles FS itself)
- Handling `RequestPermission` (opencode handles permissions itself in ACP mode)
- Multiple models per session
- Expandable/collapsible tool output in TUI (just truncate for v1)
- Removing `managed-process-migration.md` spec (keep for history)

## Open Questions

1. Does `session/prompt` block until agent is fully done, or do we need to handle a separate completion signal?
   - **Tested**: The response (with matching `id`) arrives after all notifications. Blocking read is correct.

2. Should we validate model ID against `availableModels` from `session/new` response?
   - **Answer**: No. Let `session/set_model` return an error if invalid. Simpler.

3. What happens if opencode acp process dies mid-iteration?
   - **Answer**: `readMessage()` returns EOF error, `prompt()` returns error, orchestrator handles it.

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| opencode ACP format changes | Medium | Protocol is stable (v1), test against latest opencode |
| `session/set_model` marked UNSTABLE | Medium | Method works today, monitor opencode releases |
| Notification parsing misses new update types | Low | Unknown types ignored, only text matters |
| Subprocess hangs on ACP init | Low | Context timeout kills process |

## References

- PR fixing set_model: https://github.com/anomalyco/opencode/pull/9940
- ACP Go SDK method constant: `AgentMethodSessionSetModel = "session/set_model"`
- Prior spec: `specs/managed-process-migration.md` (documents the move away from ACP)
- Tested protocol flow against opencode v1.1.33
