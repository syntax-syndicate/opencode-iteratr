# MCP Tools Server

## Overview

Replace CLI tool injection with an embedded MCP HTTP server. Agent uses native MCP tools instead of shell commands. CLI tools remain for debugging but are removed from prompt.

## User Story

As iteratr, I want to expose task/note/session tools via MCP protocol so agents can use native tool discovery and structured responses instead of spawning CLI processes.

## Requirements

- Start MCP HTTP server on random port when session begins
- Register 8 tools (consolidated from CLI):
  - **task-add**: array of {content, status?} - add one or more tasks
  - **task-update**: {id, status?, priority?, depends_on?} - update task, empty fields unchanged
  - **task-list**: list all tasks grouped by status
  - **task-next**: get next highest priority unblocked task
  - **note-add**: array of {content, type} - add one or more notes
  - **note-list**: list notes, optional type filter
  - **iteration-summary**: record summary for current iteration
  - **session-complete**: mark session as complete
- Pass server URL to ACP session/new via McpServers config
- Return errors as text content (agent can interpret and retry)
- Remove CLI tool instructions and tool table from template (agent discovers via MCP)
- Keep `iteratr tool` CLI subcommands for debugging/manual use

## Technical Implementation

### Package: `internal/mcpserver/`

**server.go** - Server lifecycle:
```go
type Server struct {
    store      *session.Store
    sessName   string
    mcpServer  *server.MCPServer
    httpServer *server.StreamableHTTPServer
    port       int
    mu         sync.Mutex
}

func New(store *session.Store, sessionName string) *Server
func (s *Server) Start(ctx context.Context) (int, error)  // Returns port
func (s *Server) Stop() error
func (s *Server) URL() string  // http://localhost:PORT/mcp
```

**tools.go** - Tool registration using mcp-go schema builders:

```go
// task-add: array of task objects
mcp.NewTool("task-add",
    mcp.WithDescription("Add one or more tasks"),
    mcp.WithArray("tasks", mcp.Required(),
        mcp.Items(map[string]any{
            "type": "object",
            "properties": map[string]any{
                "content": map[string]any{"type": "string"},
                "status":  map[string]any{"type": "string"},
            },
            "required": []string{"content"},
        })))

// task-update: id required, other fields optional
mcp.NewTool("task-update",
    mcp.WithDescription("Update task status, priority, or dependencies"),
    mcp.WithString("id", mcp.Required()),
    mcp.WithString("status"),
    mcp.WithNumber("priority"),
    mcp.WithString("depends_on"))

// note-add: array of note objects
mcp.NewTool("note-add",
    mcp.WithDescription("Add one or more notes"),
    mcp.WithArray("notes", mcp.Required(),
        mcp.Items(map[string]any{
            "type": "object",
            "properties": map[string]any{
                "content": map[string]any{"type": "string"},
                "type":    map[string]any{"type": "string"},
            },
            "required": []string{"content", "type"},
        })))
```

Other tools use simple string params with Required()/Description()

**handlers.go** - Tool handlers:
- Port logic from `cmd/iteratr/tool.go`
- Return `mcp.NewToolResultText()` for success
- Return error text (not MCP error) for failures so agent can retry
- Array params (tasks, notes) come as `[]any` from mcp-go, type-assert to `[]map[string]any`
- `handleTaskUpdate` calls store.TaskStatus/TaskPriority/TaskDepends conditionally based on which fields are set
- `handleNoteAdd` loops and calls store.NoteAdd for each note (no batch method exists)

### ACP Changes (`internal/agent/acp.go`)

Add MCP server config types:
```go
type McpServer struct {
    Type    string       `json:"type"`    // "http"
    Name    string       `json:"name"`    // "iteratr-tools"
    URL     string       `json:"url"`     // "http://localhost:PORT/mcp"
    Headers []HttpHeader `json:"headers"` // empty array
}

type HttpHeader struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

Update signatures:
- `newSession(ctx, cwd, mcpURL string)` - include McpServers in params
- `LoadSession(ctx, sessionID, cwd, mcpURL string)` - same for session resume
- Current `McpServers: []any{}` becomes typed `[]McpServer`

### Runner Changes (`internal/agent/runner.go`)

- Add `MCPServerURL` to `RunnerConfig`
- Pass URL to `newSession` calls

### Orchestrator Changes (`internal/orchestrator/orchestrator.go`)

- Add `mcpServer *mcpserver.Server` field
- Start MCP server in `Start()` after JetStream setup (needs `store` and `sessionName`)
- Store URL for passing to RunnerConfig in `Run()`
- Stop MCP server in `Stop()` **after** runner stops (runner may still be using it)

### Template Changes (`internal/template/default.go`)

Remove:
- Rule: "Use `{{binary}} tool` for ALL task management" 
- Workflow: CLI-specific steps like `task-status --id X --status in_progress`
- Commands section: entire `{{binary}} tool COMMAND...` format and table
- If Stuck: CLI examples like `note-add --type stuck --content "..."`

Keep:
- Rules: "ONE task per iteration", "Test changes", "Write iteration-summary", etc.
- Workflow: conceptual steps (sync tasks, pick task, implement, mark complete, summarize)
- If Stuck: conceptual guidance (mark blocked, use dependencies)
- Subagents: entire section unchanged

Update wording to be MCP-generic (e.g., "use task-status tool" instead of CLI invocation)

## Tasks

### 1. Create MCP server package
- [ ] Create `internal/mcpserver/server.go` with Server struct, New, Start, Stop, URL
- [ ] Create `internal/mcpserver/tools.go` with registerTools method
- [ ] Create `internal/mcpserver/handlers.go` with all tool handlers

### 2. Implement tool handlers
- [ ] Implement handleTaskAdd (parse []any from mcp-go, call TaskBatchAdd)
- [ ] Implement handleTaskUpdate (conditionally call TaskStatus/Priority/Depends)
- [ ] Implement handleTaskList, handleTaskNext
- [ ] Implement handleNoteAdd (parse []any, loop NoteAdd)
- [ ] Implement handleNoteList
- [ ] Implement handleIterationSummary, handleSessionComplete

### 3. Integrate with orchestrator
- [ ] Add mcpServer field to Orchestrator struct
- [ ] Start MCP server in Orchestrator.Start(), log port
- [ ] Pass MCP URL to Runner via config
- [ ] Stop MCP server in Orchestrator.Stop()

### 4. Update ACP protocol
- [ ] Add McpServer and HttpHeader types to acp.go (bottom with other types)
- [ ] Update newSessionParams to use `[]McpServer` instead of `[]any`
- [ ] Update loadSessionParams similarly
- [ ] Update newSession signature: `newSession(ctx, cwd, mcpURL string)`
- [ ] Update LoadSession signature: `LoadSession(ctx, sessionID, cwd, mcpURL string)`
- [ ] Build McpServer slice from mcpURL in both functions

### 5. Update runner
- [ ] Add MCPServerURL to RunnerConfig
- [ ] Pass URL when calling newSession

### 6. Update template
- [ ] Remove CLI tool command format from template
- [ ] Remove tool reference table
- [ ] Update workflow to reference MCP tools generically

### 7. Testing
- [ ] Unit tests for MCP server Start/Stop
- [ ] Unit tests for each tool handler
- [ ] Integration test: start server, call tools via HTTP

## Out of Scope

- stdio transport (may add later for subprocess use cases)
- HTTPS/TLS (localhost only for now)
- Removing `iteratr tool` CLI commands (kept for debugging)
- Authentication/authorization (trusted localhost)

## Gotchas

1. **Server readiness**: `Start()` must block until HTTP server is actually listening, otherwise runner may call MCP tools before server is ready
2. **Stop order**: In `Stop()`, stop runner first, then MCP server. Runner may still call tools during graceful shutdown
3. **Context propagation**: Pass orchestrator's context to MCP server Start() for coordinated cancellation
4. **Error format**: All tool errors return as text content (not MCP protocol errors) so agent can parse and retry. Format: `"error: <message>"`
5. **Empty headers array**: ACP requires `headers: []` even when empty, not `null`
