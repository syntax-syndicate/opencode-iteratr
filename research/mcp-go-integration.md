# MCP-Go Integration Research

Replace CLI tool injection (`iteratr tool task-xxx`) with an embedded MCP HTTP server.

## Current Architecture

### Tool Injection via Template
Tools are injected as CLI commands in the prompt template (`internal/template/default.go`):

```
{{binary}} tool COMMAND --data-dir .iteratr --name {{session}} [args]
```

Commands: `task-add`, `task-batch-add`, `task-status`, `task-priority`, `task-depends`, `task-list`, `task-next`, `note-add`, `note-list`, `iteration-summary`, `session-complete`

### ACP Session Init
`internal/agent/acp.go` passes empty McpServers array:

```go
params := newSessionParams{
    Cwd:        cwd,
    McpServers: []any{},  // <-- Currently empty
}
```

### Tool Commands
`cmd/iteratr/tool.go` implements CLI subcommands connecting to NATS via port file.

---

## Target Architecture

### Flow
```
iteratr build starts
    |
    v
Start MCP HTTP Server (random port)
    |
    v
Pass server config to ACP session/new
    |
    v
Agent uses MCP tools directly (no CLI)
```

### Benefits
- No shell spawning overhead
- Native MCP tool discovery
- Proper error handling via MCP protocol
- Tool schemas visible to agent via `tools/list`

---

## Implementation Plan

### 1. Create MCP Server Package

**File**: `internal/mcpserver/server.go`

```go
package mcpserver

import (
    "context"
    "fmt"
    "net"
    "sync"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/mark3labs/iteratr/internal/session"
)

// Server wraps the MCP HTTP server.
type Server struct {
    store      *session.Store
    sessName   string
    mcpServer  *server.MCPServer
    httpServer *server.StreamableHTTPServer
    port       int
    mu         sync.Mutex
}

// New creates an MCP server with all iteratr tools registered.
func New(store *session.Store, sessionName string) *Server {
    s := &Server{
        store:    store,
        sessName: sessionName,
    }
    s.mcpServer = server.NewMCPServer(
        "iteratr-tools",
        "1.0.0",
        server.WithToolCapabilities(true),
    )
    s.registerTools()
    return s
}

// Start launches the HTTP server on a random available port.
// Returns the port number for client configuration.
func (s *Server) Start(ctx context.Context) (int, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Find available port
    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        return 0, fmt.Errorf("failed to find available port: %w", err)
    }
    port := listener.Addr().(*net.TCPAddr).Port
    listener.Close()

    s.port = port
    s.httpServer = server.NewStreamableHTTPServer(
        s.mcpServer,
        server.WithEndpointPath("/mcp"),
    )

    // Start in goroutine
    go func() {
        addr := fmt.Sprintf(":%d", port)
        if err := s.httpServer.Start(addr); err != nil {
            // Log error - server already closed is expected on shutdown
        }
    }()

    return port, nil
}

// Stop shuts down the MCP server.
func (s *Server) Stop() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.httpServer != nil {
        return s.httpServer.Shutdown(context.Background())
    }
    return nil
}

// URL returns the full MCP server URL.
func (s *Server) URL() string {
    return fmt.Sprintf("http://localhost:%d/mcp", s.port)
}
```

### 2. Register Tools

**File**: `internal/mcpserver/tools.go`

```go
package mcpserver

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/iteratr/internal/session"
)

func (s *Server) registerTools() {
    // task-add
    s.mcpServer.AddTool(
        mcp.NewTool("task-add",
            mcp.WithDescription("Add a new task to the session"),
            mcp.WithString("content",
                mcp.Required(),
                mcp.Description("Task content/description"),
            ),
            mcp.WithString("status",
                mcp.Description("Initial status (remaining, in_progress, completed, blocked)"),
            ),
        ),
        s.handleTaskAdd,
    )

    // task-batch-add
    s.mcpServer.AddTool(
        mcp.NewTool("task-batch-add",
            mcp.WithDescription("Add multiple tasks at once"),
            mcp.WithString("tasks",
                mcp.Required(),
                mcp.Description(`JSON array of tasks: [{"content":"...", "status":"remaining"}]`),
            ),
        ),
        s.handleTaskBatchAdd,
    )

    // task-status
    s.mcpServer.AddTool(
        mcp.NewTool("task-status",
            mcp.WithDescription("Update task status"),
            mcp.WithString("id", mcp.Required(), mcp.Description("Task ID")),
            mcp.WithString("status",
                mcp.Required(),
                mcp.Description("New status: remaining, in_progress, completed, blocked"),
            ),
        ),
        s.handleTaskStatus,
    )

    // task-priority
    s.mcpServer.AddTool(
        mcp.NewTool("task-priority",
            mcp.WithDescription("Update task priority"),
            mcp.WithString("id", mcp.Required(), mcp.Description("Task ID")),
            mcp.WithNumber("priority",
                mcp.Required(),
                mcp.Description("Priority: 0=critical, 1=high, 2=medium, 3=low, 4=backlog"),
            ),
        ),
        s.handleTaskPriority,
    )

    // task-depends
    s.mcpServer.AddTool(
        mcp.NewTool("task-depends",
            mcp.WithDescription("Add task dependency"),
            mcp.WithString("id", mcp.Required(), mcp.Description("Task ID")),
            mcp.WithString("depends_on",
                mcp.Required(),
                mcp.Description("ID of task this depends on"),
            ),
        ),
        s.handleTaskDepends,
    )

    // task-list
    s.mcpServer.AddTool(
        mcp.NewTool("task-list",
            mcp.WithDescription("List all tasks grouped by status"),
        ),
        s.handleTaskList,
    )

    // task-next
    s.mcpServer.AddTool(
        mcp.NewTool("task-next",
            mcp.WithDescription("Get the next highest priority unblocked task"),
        ),
        s.handleTaskNext,
    )

    // note-add
    s.mcpServer.AddTool(
        mcp.NewTool("note-add",
            mcp.WithDescription("Add a note to the session"),
            mcp.WithString("content", mcp.Required(), mcp.Description("Note content")),
            mcp.WithString("type",
                mcp.Required(),
                mcp.Description("Note type: learning, stuck, tip, decision"),
            ),
        ),
        s.handleNoteAdd,
    )

    // note-list
    s.mcpServer.AddTool(
        mcp.NewTool("note-list",
            mcp.WithDescription("List notes"),
            mcp.WithString("type", mcp.Description("Filter by type")),
        ),
        s.handleNoteList,
    )

    // iteration-summary
    s.mcpServer.AddTool(
        mcp.NewTool("iteration-summary",
            mcp.WithDescription("Record a summary for the current iteration"),
            mcp.WithString("summary",
                mcp.Required(),
                mcp.Description("Summary of what was accomplished"),
            ),
        ),
        s.handleIterationSummary,
    )

    // session-complete
    s.mcpServer.AddTool(
        mcp.NewTool("session-complete",
            mcp.WithDescription("Mark session as complete (only when ALL tasks done)"),
        ),
        s.handleSessionComplete,
    )
}
```

### 3. Tool Handlers

```go
func (s *Server) handleTaskAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    content := req.GetString("content", "")
    status := req.GetString("status", "remaining")

    if content == "" {
        return nil, fmt.Errorf("content is required")
    }

    task, err := s.store.TaskAdd(ctx, s.sessName, session.TaskAddParams{
        Content: content,
        Status:  status,
    })
    if err != nil {
        return nil, err
    }

    output, _ := json.Marshal(map[string]string{
        "id":      task.ID,
        "status":  task.Status,
        "content": task.Content,
    })
    return mcp.NewToolResultText(string(output)), nil
}

func (s *Server) handleTaskList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    result, err := s.store.TaskList(ctx, s.sessName)
    if err != nil {
        return nil, err
    }

    // Format as text for agent consumption
    var lines []string
    formatTasks := func(status string, tasks []*session.Task) {
        if len(tasks) == 0 {
            return
        }
        lines = append(lines, fmt.Sprintf("%s:", status))
        for _, t := range tasks {
            lines = append(lines, fmt.Sprintf("  [%s] %s", t.ID, t.Content))
        }
    }

    formatTasks("Remaining", result.Remaining)
    formatTasks("In progress", result.InProgress)
    formatTasks("Completed", result.Completed)
    formatTasks("Blocked", result.Blocked)

    if len(lines) == 0 {
        return mcp.NewToolResultText("No tasks"), nil
    }
    return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// ... implement other handlers similarly ...
```

### 4. Integrate with Orchestrator

**File**: `internal/orchestrator/orchestrator.go`

```go
type Orchestrator struct {
    // ... existing fields ...
    mcpServer *mcpserver.Server
}

func (o *Orchestrator) Start() error {
    // ... existing NATS setup ...

    // Start MCP server
    o.mcpServer = mcpserver.New(o.store, o.config.SessionName)
    mcpPort, err := o.mcpServer.Start(ctx)
    if err != nil {
        return fmt.Errorf("failed to start MCP server: %w", err)
    }
    logger.Info("MCP server started on port %d", mcpPort)

    // Pass MCP config to runner
    o.runner = agent.NewRunner(agent.RunnerConfig{
        // ... existing config ...
        MCPServerURL: o.mcpServer.URL(),
    })

    // ... rest of setup ...
}

func (o *Orchestrator) Stop() error {
    // ... existing cleanup ...
    if o.mcpServer != nil {
        o.mcpServer.Stop()
    }
    return nil
}
```

### 5. Update ACP Session Init

**File**: `internal/agent/acp.go`

```go
type newSessionParams struct {
    Cwd        string     `json:"cwd"`
    McpServers []McpServer `json:"mcpServers"`
}

// McpServer represents an MCP server configuration for ACP.
// For HTTP transport (type="http" discriminator).
type McpServer struct {
    Type    string      `json:"type"`    // "http"
    Name    string      `json:"name"`    // "iteratr-tools"
    URL     string      `json:"url"`     // "http://localhost:PORT/mcp"
    Headers []HttpHeader `json:"headers"` // can be empty
}

type HttpHeader struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

// newSession creates a new ACP session with MCP server configured.
func (c *acpConn) newSession(ctx context.Context, cwd string, mcpURL string) (string, error) {
    var mcpServers []McpServer
    if mcpURL != "" {
        mcpServers = []McpServer{{
            Type:    "http",
            Name:    "iteratr-tools",
            URL:     mcpURL,
            Headers: []HttpHeader{}, // Required but can be empty
        }}
    }

    params := newSessionParams{
        Cwd:        cwd,
        McpServers: mcpServers,
    }
    // ... rest unchanged ...
}
```

### 6. Update Runner

**File**: `internal/agent/runner.go`

```go
type RunnerConfig struct {
    // ... existing fields ...
    MCPServerURL string // URL for iteratr MCP tools server
}

type Runner struct {
    // ... existing fields ...
    mcpServerURL string
}

func NewRunner(cfg RunnerConfig) *Runner {
    return &Runner{
        // ... existing fields ...
        mcpServerURL: cfg.MCPServerURL,
    }
}

func (r *Runner) RunIteration(ctx context.Context, prompt string, hookOutput string) error {
    // ... existing code ...

    // Create fresh session with MCP server configured
    sessID, err := r.conn.newSession(ctx, r.workDir, r.mcpServerURL)
    // ... rest unchanged ...
}
```

### 7. Update Template

**File**: `internal/template/default.go`

Remove CLI tool instructions, replace with MCP tool guidance:

```go
const DefaultTemplate = `# iteratr Session
Session: {{session}} | Iteration: #{{iteration}}

{{history}}

## Spec
{{spec}}

{{tasks}}

{{notes}}

## Rules
- ONE task per iteration - complete fully, then STOP
- Use the iteratr-tools MCP server for ALL task management
- Test changes before marking complete
- Write iteration-summary before stopping
- Call session-complete only when ALL tasks done
- Respect user-added tasks even if not in spec

## Workflow
1. If no tasks: sync from spec via task-batch-add
2. Pick ONE ready task (highest priority, no blockers)
3. task-status(id, "in_progress")
4. Implement + test
5. task-status(id, "completed")
6. iteration-summary(summary)
7. STOP (do not pick another task)

## Available Tools (iteratr-tools MCP server)
| Tool | Args | Notes |
|------|------|-------|
| task-add | content, status? | Add single task |
| task-batch-add | tasks (JSON array) | Add multiple tasks |
| task-status | id, status | Status: remaining, in_progress, completed, blocked |
| task-priority | id, priority | 0=critical, 1=high, 2=medium, 3=low, 4=backlog |
| task-depends | id, depends_on | Set dependency |
| task-list | | List all tasks |
| task-next | | Get next ready task |
| note-add | content, type | Type: learning, stuck, tip, decision |
| note-list | type? | List notes |
| iteration-summary | summary | Record what you did |
| session-complete | | Only when ALL tasks done |

## If Stuck
- note-add(content, "stuck")
- Mark task blocked or fix before completing
- If blocked by another task: task-depends(id, blocker_id)

## Subagents
Spin up subagents (via Task tool) to parallelize work.
{{extra}}`
```

---

## ACP McpServers Format Reference

### HTTP Server (streamable-http)
```json
{
  "type": "http",
  "name": "iteratr-tools",
  "url": "http://localhost:45678/mcp",
  "headers": []
}
```

### SSE Server (alternative)
```json
{
  "type": "sse",
  "name": "iteratr-tools",
  "url": "http://localhost:45678/mcp",
  "headers": []
}
```

### Stdio Server (local process)
```json
{
  "name": "iteratr-tools",
  "command": "/path/to/iteratr",
  "args": ["mcp", "serve"],
  "env": []
}
```

---

## mcp-go Reference

### Create Server
```go
mcpServer := server.NewMCPServer("name", "1.0.0",
    server.WithToolCapabilities(true),
)
```

### Define Tool
```go
mcpServer.AddTool(
    mcp.NewTool("tool-name",
        mcp.WithDescription("Tool description"),
        mcp.WithString("param1", mcp.Required(), mcp.Description("...")),
        mcp.WithNumber("param2", mcp.Description("...")),
        mcp.WithBoolean("param3"),
    ),
    handler,
)
```

### Tool Handler
```go
func handler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    value := req.GetString("param1", "default")
    num := req.GetFloat("param2", 0)
    
    // Process...
    
    return mcp.NewToolResultText("result"), nil
}
```

### HTTP Server
```go
httpServer := server.NewStreamableHTTPServer(mcpServer,
    server.WithEndpointPath("/mcp"),
    server.WithHeartbeatInterval(30),
)

// Random port
listener, _ := net.Listen("tcp", ":0")
port := listener.Addr().(*net.TCPAddr).Port
listener.Close()

httpServer.Start(fmt.Sprintf(":%d", port))
```

### Shutdown
```go
httpServer.Shutdown(ctx)
```

---

## Implementation Tasks

### 1. Create MCP Server Package
- [ ] `internal/mcpserver/server.go` - Server struct, Start/Stop/URL
- [ ] `internal/mcpserver/tools.go` - Tool registration
- [ ] `internal/mcpserver/handlers.go` - Tool handlers (port from tool.go)

### 2. Integrate with Orchestrator
- [ ] Add mcpServer field to Orchestrator
- [ ] Start MCP server in Orchestrator.Start()
- [ ] Pass URL to Runner
- [ ] Stop MCP server in Orchestrator.Stop()

### 3. Update ACP Protocol
- [ ] Add McpServer type to types.go
- [ ] Update newSessionParams to include typed McpServers
- [ ] Update newSession to accept mcpURL parameter
- [ ] Update LoadSession similarly

### 4. Update Runner
- [ ] Add MCPServerURL to RunnerConfig
- [ ] Pass URL to newSession calls

### 5. Update Template
- [ ] Remove CLI tool command instructions
- [ ] Add MCP tool usage instructions
- [ ] Keep tool reference table updated

### 6. Testing
- [ ] Unit tests for MCP server
- [ ] Integration test: start server, call tools via MCP client
- [ ] E2E test: full build loop with MCP tools

### 7. Cleanup
- [ ] Consider deprecating `iteratr tool` CLI commands
- [ ] Or keep them for debugging/manual use

---

## Dependencies

Add to `go.mod`:
```
github.com/mark3labs/mcp-go v0.26.0  // or latest
```

Import:
```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)
```

---

## Open Questions

1. **Session name passing**: MCP tools need session name context. Options:
   - Embed in server at creation time (current plan)
   - Pass via tool parameters (more flexible but verbose)
   - Use MCP session context (if supported)

2. **Error handling**: How should tool errors appear to agent?
   - MCP error response (agent sees error)
   - Text result with error message (agent can interpret)
   - Both depending on error type

3. **Backward compatibility**: Keep CLI tools?
   - Useful for debugging
   - Users may have scripts depending on them
   - Consider deprecation notice

4. **HTTPS**: Need TLS for production deployments?
   - localhost-only initially is fine
   - mcp-go supports `WithTLSCert()`
