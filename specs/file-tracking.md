# File Tracking

Track files modified during an iteration for auto-commit functionality.

## Overview

Parse ACP `tool_call_update` notifications for completed edit operations, extract file paths and diff content, maintain per-iteration modified file set, enable optional auto-commit at iteration end.

## User Story

**As a** developer using iteratr  
**I want** automatic tracking of files modified by the agent  
**So that** I can auto-commit changes after each iteration without manually identifying what changed

## Requirements

### Functional

1. **Track Edit Tool Completions**
   - Detect `tool_call_update` with `status: "completed"` and `kind: "edit"`
   - Extract file path and isNew from `content[]` diff blocks (primary) or metadata (fallback)
   - Extract additions/deletions counts from `rawOutput.metadata.filediff` when available

2. **Modified Files Set**
   - Maintain per-iteration set of modified file paths
   - Track whether file was created (oldText empty) vs modified
   - Track additions/deletions counts when available from metadata
   - Clear set at iteration start

3. **Auto-Commit (Optional)**
   - CLI flag: `--auto-commit` (default: true), use `--auto-commit=false` to disable
   - At iteration end, if modified files exist and flag enabled:
     - Send commit prompt to existing ACP session via `runner.SendMessages()`
     - Prompt includes: list of modified files, task context, iteration summary
     - Agent generates meaningful commit message and runs git commands
   - Skip if no files modified or not in git repo

4. **Commit Prompt**
   - Reuse existing ACP session via `runner.SendMessages()` (faster, session has iteration context)
   - Prompt includes modified files list with +/-  counts, current task, iteration summary
   - Agent generates commit message based on actual changes and context
   - Example prompt: "Commit these modified files: auth.go (+15/-3), session.go (new file). Task: implement JWT validation"

5. **TUI Display**
   - Show modified files count in status bar during iteration
   - After iteration, show summary: "Modified 3 files" with option to view list

6. **Remove Manual Commit Instructions from Prompts**
   - DefaultTemplate and .iteratr.template currently instruct agent to manually commit
   - With auto-commit, these instructions become redundant/conflicting
   - Remove commit step from workflow, revert instructions, and "commit before completing" rule

### Non-Functional

1. File paths normalized to relative paths from working directory
2. Duplicate paths deduplicated (same file edited multiple times)
3. No tracking for read-only tools (read, glob, grep)
4. No tracking for failed edits (`status: "error"` or `status: "canceled"`)
5. Graceful handling if git not available (skip auto-commit, warn)

## Technical Implementation

### Data Structures

```go
// internal/agent/filetracker.go

// FileChange represents a single file modification
type FileChange struct {
    Path      string // Relative path from working directory
    AbsPath   string // Absolute path
    IsNew     bool   // True if file was created (oldText was empty at extraction)
    Additions int    // Lines added (from metadata, 0 if unknown)
    Deletions int    // Lines deleted (from metadata, 0 if unknown)
}

// FileTracker tracks files modified during an iteration
type FileTracker struct {
    workDir  string
    changes  map[string]*FileChange // keyed by relative path
    mu       sync.Mutex
}

func NewFileTracker(workDir string) *FileTracker
func (ft *FileTracker) RecordChange(absPath string, isNew bool, additions, deletions int)
func (ft *FileTracker) Get(relPath string) *FileChange   // Get change by relative path, nil if not found
func (ft *FileTracker) Changes() []*FileChange           // Returns sorted list
func (ft *FileTracker) ModifiedPaths() []string          // Just paths, sorted
func (ft *FileTracker) Clear()                           // Reset for new iteration
func (ft *FileTracker) Count() int                       // Number of unique files
func (ft *FileTracker) HasChanges() bool                 // Any files modified?
```

### ACP Integration

Enhance `toolCallUpdate` parsing to extract diff content blocks.

**NOTE**: This REPLACES the existing `toolCallContent` struct in `acp.go:618-621`. The existing struct only handles `type: "content"` blocks. We need polymorphic handling for both content and diff blocks.

```go
// internal/agent/acp.go - REPLACE existing toolCallContent struct
// The existing struct only handles type="content", but ACP also sends type="diff"

// toolCallContent handles polymorphic content blocks in tool_call_update
// Can be either: {type:"content", content:{type:"text", text:"..."}}
//            or: {type:"diff", path:"...", oldText:"...", newText:"..."}
type toolCallContent struct {
    Type    string      `json:"type"`              // "content" or "diff"
    Content contentPart `json:"content,omitempty"` // for type="content"
    Path    string      `json:"path,omitempty"`    // for type="diff"
    OldText string      `json:"oldText,omitempty"` // for type="diff"
    NewText string      `json:"newText,omitempty"` // for type="diff"
}
```

### Callback Enhancement

Add file change callback to runner:

```go
// internal/agent/runner.go

type RunnerConfig struct {
    // ...existing fields...
    OnFileChange func(change FileChange) // Called for each file modification
}
```

### Orchestrator Integration

```go
// internal/orchestrator/orchestrator.go

type Orchestrator struct {
    // ...existing fields...
    fileTracker *agent.FileTracker
    autoCommit  bool
}

func (o *Orchestrator) Run(ctx context.Context) error {
    // Clear tracker at iteration start
    o.fileTracker.Clear()
    
    // ... run iteration ...
    
    // After iteration completes successfully
    if o.autoCommit && o.fileTracker.HasChanges() {
        if err := o.runAutoCommit(ctx); err != nil {
            logger.Warn("Auto-commit failed: %v", err)
        }
    }
}

func (o *Orchestrator) runAutoCommit(ctx context.Context) error {
    // Check if in git repo
    if !isGitRepo(o.cfg.WorkDir) {
        return nil
    }
    
    // Build commit prompt
    prompt := o.buildCommitPrompt(ctx)
    
    // Reuse existing Runner - send commit prompt to current ACP session
    // This is faster than spawning a new subprocess and the session already
    // has context about what work was done
    logger.Info("Running auto-commit via existing ACP session")
    return o.runner.SendMessages(ctx, []string{prompt})
}

func (o *Orchestrator) buildCommitPrompt(ctx context.Context) string {
    paths := o.fileTracker.ModifiedPaths()
    
    // Get context from session store
    state, _ := o.store.LoadState(ctx, o.cfg.SessionName)
    
    // Find in_progress task (if any) for context
    var currentTask string
    for _, t := range state.Tasks {
        if t.Status == "in_progress" {
            currentTask = t.Content
            break
        }
    }
    
    // Get latest iteration summary (if any)
    var iterationSummary string
    if len(state.Iterations) > 0 {
        lastIter := state.Iterations[len(state.Iterations)-1]
        iterationSummary = lastIter.Summary
    }
    
    var sb strings.Builder
    sb.WriteString("Commit the following modified files:\n\n")
    for _, p := range paths {
        change := o.fileTracker.Get(p)
        if change.IsNew {
            sb.WriteString(fmt.Sprintf("- %s (new file)\n", p))
        } else {
            sb.WriteString(fmt.Sprintf("- %s (+%d/-%d)\n", p, change.Additions, change.Deletions))
        }
    }
    sb.WriteString("\nContext:\n")
    if currentTask != "" {
        sb.WriteString(fmt.Sprintf("- Task: %s\n", currentTask))
    }
    if iterationSummary != "" {
        sb.WriteString(fmt.Sprintf("- Summary: %s\n", iterationSummary))
    }
    sb.WriteString("\nInstructions:\n")
    sb.WriteString("1. Stage only the listed files with `git add`\n")
    sb.WriteString("2. Create a commit with a clear, conventional message\n")
    sb.WriteString("3. Do NOT push\n")
    return sb.String()
}
```

### TUI Integration

```go
// internal/tui/app.go

// FileChangeMsg sent when a file is modified
type FileChangeMsg struct {
    Path      string
    IsNew     bool
    Additions int
    Deletions int
}

// Update status bar to show modified file count
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case FileChangeMsg:
        a.modifiedFileCount++
        // Update status bar
    }
}
```

### CLI Flag

```go
// cmd/build.go

var autoCommitFlag bool

func init() {
    buildCmd.Flags().BoolVar(&autoCommitFlag, "auto-commit", true, "Auto-commit modified files after iteration (use --auto-commit=false to disable)")
}
```

### Wire Format Examples (from testing)

**Write tool completed:**
```json
{
  "sessionUpdate": "tool_call_update",
  "toolCallId": "call_xxx",
  "status": "completed",
  "kind": "edit",
  "title": "research/test-file-1.txt",
  "rawInput": {
    "content": "Hello World",
    "filePath": "/abs/path/research/test-file-1.txt"
  },
  "rawOutput": {
    "metadata": {
      "filepath": "/abs/path/research/test-file-1.txt",
      "exists": false
    }
  },
  "content": [
    {"type": "content", "content": {"type": "text", "text": "Wrote file successfully."}},
    {"type": "diff", "path": "/abs/path/research/test-file-1.txt", "oldText": "", "newText": "Hello World"}
  ]
}
```

**Edit tool completed:**
```json
{
  "sessionUpdate": "tool_call_update",
  "toolCallId": "call_xxx",
  "status": "completed",
  "kind": "edit",
  "title": "research/test-file-1.txt",
  "rawInput": {
    "filePath": "/abs/path/research/test-file-1.txt",
    "oldString": "Line 1",
    "newString": "Line 1\nLine 2"
  },
  "rawOutput": {
    "metadata": {
      "filediff": {
        "file": "/abs/path/research/test-file-1.txt",
        "before": "Line 1",
        "after": "Line 1\nLine 2",
        "additions": 2,
        "deletions": 1
      }
    }
  },
  "content": [
    {"type": "content", "content": {"type": "text", "text": "Edit applied successfully."}},
    {"type": "diff", "path": "/abs/path/research/test-file-1.txt", "oldText": "Line 1", "newText": "Line 1\nLine 2"}
  ]
}
```

### Diff Extraction by Tool Type

**IMPORTANT**: Write and Edit tools have different metadata structures:

| Tool  | `content[]` diff blocks | `rawOutput.metadata.filediff` |
|-------|------------------------|-------------------------------|
| Write | YES - primary source   | NO - only has `filepath`      |
| Edit  | YES - primary source   | YES - has additions/deletions |

**Primary extraction**: Use `content[]` blocks with `type: "diff"`. Extract `path` and determine `isNew` from `oldText == ""`. Present for BOTH write and edit tools.

**Secondary extraction (edit only)**: For edit tools, `rawOutput.metadata.filediff` provides `additions`/`deletions` counts. Merge these into the FileChange if available. Write tools don't have these counts.

**Fallback for path only** (if content blocks missing):
1. `rawOutput.metadata.filediff.file` (edit tool)
2. `rawOutput.metadata.filepath` (write tool)
3. `rawInput.filePath`

### Commit Prompt Example

```
Commit the following modified files:

- internal/auth/jwt.go (new file)
- internal/auth/session.go (+15/-3)
- internal/handler/login.go (+8/-2)

Context:
- Task: implement-jwt-auth
- Summary: Added JWT validation and integrated with login handler

Instructions:
1. Stage only the listed files with `git add`
2. Create a commit with a clear, conventional message
3. Do NOT push
```

Agent generates appropriate commit message based on actual changes and context.

## Tasks

### 1. Create FileTracker Package
- [ ] Create `internal/agent/filetracker.go`
- [ ] Implement `FileChange` struct (Path, AbsPath, IsNew, Additions, Deletions)
- [ ] Implement `FileTracker` with thread-safe map
- [ ] Implement `RecordChange(absPath, isNew, additions, deletions)` - normalize path, store/update change
- [ ] Implement `Get(relPath)` - return FileChange by relative path or nil
- [ ] Implement `Changes()`, `ModifiedPaths()`, `Clear()`, `Count()`, `HasChanges()`
- [ ] Add unit tests for path normalization and deduplication

### 2. Enhance ACP Diff Extraction
- [ ] REPLACE existing `toolCallContent` struct to support both `type: "content"` and `type: "diff"` (polymorphic)
- [ ] Modify `tool_call_update` parsing in `prompt()` to extract diff blocks from content array
- [ ] Add helper to extract: path, isNew (oldText == ""), additions/deletions (from metadata)
- [ ] Only process when `status == "completed"` AND `kind == "edit"` (skip errors)
- [ ] Emit file change data via callback for each diff block found

### 3. Add OnFileChange Callback to Runner
- [ ] Add `OnFileChange func(FileChange)` to `RunnerConfig`
- [ ] Wire callback in `prompt()` when processing completed edit tool calls
- [ ] Call callback for each diff block in completed tool_call_update

### 4. Integrate FileTracker into Orchestrator
- [ ] Add `fileTracker *agent.FileTracker` field to Orchestrator
- [ ] Add `autoCommit bool` field to Orchestrator
- [ ] Initialize FileTracker in orchestrator constructor
- [ ] Wire `OnFileChange` callback to record changes in tracker
- [ ] Clear tracker at start of each iteration

### 5. Implement Auto-Commit via Existing Runner
- [ ] Add `isGitRepo(dir string) bool` helper (check for .git directory)
- [ ] Add `runAutoCommit(ctx context.Context) error` method to Orchestrator
- [ ] Implement `buildCommitPrompt(ctx)` - generate prompt with file list, task context from store
- [ ] Reuse existing Runner via `SendMessages()` (faster than new subprocess, session has context)
- [ ] Call runAutoCommit at iteration end if autoCommit enabled and changes exist
- [ ] Handle commit failures gracefully (log warning, continue - don't block iteration flow)

### 6. Add CLI Flag
- [ ] Add `--auto-commit` flag to `build` command (default: true)
- [ ] Pass flag value to orchestrator config
- [ ] Add flag to `--help` output with description

### 7. TUI Integration
- [ ] Add `FileChangeMsg` message type to app.go
- [ ] Track modified file count in App state
- [ ] Update status bar to show "N files modified" during iteration
- [ ] Send FileChangeMsg from orchestrator callback (TUI mode)

### 8. Remove Git Commit Instructions from Prompts
- [ ] Update `internal/template/default.go` DefaultTemplate:
  - Remove step 5 "Commit - commit changes with a clear message" from Workflow
  - Remove "Need to Revert Changes" section (git checkout/revert instructions)
  - Remove "Commit before completing" rule
- [ ] Update `.iteratr.template`:
  - Remove step 5 "Commit - commit changes with a clear message" from Workflow
  - Remove "Need to Revert Changes" section
  - Remove "Commit before completing" rule

### 9. Testing
- [ ] Unit test FileTracker: RecordChange, dedup, path normalization
- [ ] Unit test diff extraction from various content block formats
- [ ] Unit test buildCommitPrompt() generates correct prompt format
- [ ] Integration test: run iteration, verify files tracked correctly
- [ ] Manual test: --auto-commit sends prompt to existing session and creates commit
- [ ] Manual test: commit message is contextually appropriate
- [ ] Manual test: TUI shows modified file count

## Prompt Template Changes

With auto-commit, the agent no longer needs to manually commit. Remove these instructions:

**From Workflow section:**
- Remove: `5. **Commit** - commit changes with a clear message`
- Renumber steps 6-9 to 5-8

**From "If Something Goes Wrong" section:**
- Remove entire "Need to Revert Changes" subsection (git checkout/revert)

**From Rules section:**
- Remove: `- **Commit before completing** - always commit changes before marking a task completed`

**Files affected:**
- `internal/template/default.go` (DefaultTemplate constant)
- `.iteratr.template` (user-facing template file)

## Out of Scope

- Tracking file deletions (rm command via bash)
- Tracking files modified by subagents (tracked separately by their session)
- Undo/revert functionality
- Staging partial changes (hunk-level)
- Pre-commit hooks integration
- Tracking non-text files (images, binaries)
- Automatic push after commit

## Open Questions

1. Should auto-commit be opt-in (flag) or opt-out (default on)?
   - **Decision**: Opt-out via `--auto-commit=false`. Default on for convenience.

2. Should we track file content (oldText/newText)?
   - **Decision**: No. Only track path, isNew, additions/deletions. Agent has session context already.

3. What if git add/commit fails mid-way?
   - **Decision**: Log warning, continue. Don't block iteration flow.

4. Should we support `.gitignore` filtering?
   - **Decision**: No. Git add will respect gitignore automatically.

## References

- Research: `research/acp-file-tracking-test.ts` - ACP file operation test script
- Research: `research/opencode-acp-protocol.md` - ACP protocol documentation
- Crush pattern: `filetracker` package for read/write time tracking
- ACP spec: `specs/acp-migration.md` - tool_call_update format
