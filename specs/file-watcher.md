# File Watcher

Replace event-only file tracking with fsnotify filesystem watcher for robust modified file detection.

## Overview

Current file tracking parses ACP `tool_call_update` events — only catches files modified through agent edit/write tools. Files modified via bash commands, subprocesses, patches, or any non-tool mechanism are invisible. Add fsnotify-based watcher that catches all filesystem changes regardless of source.

## User Story

**As a** developer using iteratr
**I want** all file modifications detected automatically
**So that** auto-commit captures everything the agent changed, not just tool-based edits

## Requirements

### Functional

1. **fsnotify Watcher**
   - Watch working directory recursively (walk tree, add each subdir)
   - Dynamically add new subdirectories when created
   - Detect: Create, Write, Rename events (ignore Chmod)
   - Debounce rapid writes per-file (100ms timer)
   - Thread-safe changed file set

2. **Default Exclusions** (always excluded)
   - `.git/`
   - `node_modules/`

3. **Data Dir Exclusion** (configurable)
   - New config: `watch_data_dir` (default: `false`)
   - When false, excludes `data_dir` (default `.iteratr/`) from watcher
   - When true, data dir changes are tracked and eligible for auto-commit

4. **Hybrid Detection**
   - Keep ACP event tracking for real-time TUI updates (immediate `FileChangeMsg`)
   - FileWatcher runs as supplementary detection layer
   - At auto-commit time, union both sources:
     - ACP-tracked files retain rich metadata (isNew, additions, deletions)
     - Watcher-only files added with minimal metadata (path only)
   - FileTracker remains single source of truth for commit prompt

5. **Lifecycle**
   - Watcher starts when orchestrator `Run()` begins (once, not per-iteration)
   - Changed set cleared at iteration start (alongside FileTracker.Clear())
   - Watcher stopped on orchestrator shutdown

### Non-Functional

1. Paths normalized to relative from working directory
2. Deduplicated (same file from both ACP and watcher = one entry)
3. Graceful degradation if fsnotify fails (log warning, fall back to ACP-only)
4. inotify watch limit: log clear error message if hit

## Technical Implementation

### FileWatcher

```go
// internal/agent/filewatcher.go

type FileWatcher struct {
    watcher    *fsnotify.Watcher
    workDir    string
    excludeDirs map[string]bool // dirs to skip (relative names)
    changed    map[string]time.Time // path -> last event time (for debounce)
    mu         sync.Mutex
    done       chan struct{}
}

func NewFileWatcher(workDir string, excludeDirs []string) (*FileWatcher, error)
func (fw *FileWatcher) Start() error      // Walk tree, add watches, start event loop
func (fw *FileWatcher) Stop() error       // Close watcher, stop event loop
func (fw *FileWatcher) Clear()            // Reset changed set for new iteration
func (fw *FileWatcher) ChangedPaths() []string // Debounced changed paths (relative)
func (fw *FileWatcher) HasChanges() bool
```

Event loop goroutine:
- Read from `watcher.Events` channel
- Filter by op (Create, Write, Rename only)
- Filter by path (skip excluded dirs)
- On Create dir: `watcher.Add(path)` for new subdirectories
- Record path with timestamp in changed map
- Debounce: only include paths where `time.Since(lastEvent) > 100ms`

### FileTracker Enhancement

Add method to merge watcher paths without overwriting ACP metadata:

```go
// MergeWatcherPaths adds paths from fsnotify that aren't already tracked by ACP.
// Watcher-only files get minimal metadata (path only, no additions/deletions).
func (ft *FileTracker) MergeWatcherPaths(paths []string)
```

### Config Addition

```go
// internal/config/config.go
type Config struct {
    // ...existing fields...
    WatchDataDir bool `mapstructure:"watch_data_dir" yaml:"watch_data_dir"`
}
```

Default: `false`. ENV: `ITERATR_WATCH_DATA_DIR`.

```yaml
# iteratr.yml
watch_data_dir: false  # default: don't track data_dir changes
```

### Orchestrator Integration

```go
// In Run():
excludeDirs := []string{".git", "node_modules"}
if !o.cfg.WatchDataDir {
    excludeDirs = append(excludeDirs, o.cfg.DataDir)
}
fw, err := agent.NewFileWatcher(o.cfg.WorkDir, excludeDirs)
// Start watcher (once for session)
fw.Start()
defer fw.Stop()

// Per iteration:
o.fileTracker.Clear()
fw.Clear()

// At auto-commit time:
o.fileTracker.MergeWatcherPaths(fw.ChangedPaths())
// Then existing buildCommitPrompt() works unchanged
```

## Tasks

### 1. Add config field
- [ ] Add `WatchDataDir bool` to Config struct with default false
- [ ] Add env binding `ITERATR_WATCH_DATA_DIR`
- [ ] Update config tests

### 2. Create FileWatcher
- [ ] Create `internal/agent/filewatcher.go`
- [ ] Implement constructor with exclusion list
- [ ] Implement recursive directory walking with exclusion filtering
- [ ] Implement fsnotify event loop goroutine (Create/Write/Rename only)
- [ ] Implement dynamic subdirectory watching on Create
- [ ] Implement debouncing (100ms per-file timer)
- [ ] Implement ChangedPaths(), HasChanges(), Clear(), Stop()

### 3. Add MergeWatcherPaths to FileTracker
- [ ] Add `MergeWatcherPaths(paths []string)` — skip paths already in tracker
- [ ] Unit test: ACP metadata preserved when watcher reports same file

### 4. Wire into orchestrator
- [ ] Build exclude list from config (always .git, node_modules; conditionally data_dir)
- [ ] Create and start FileWatcher in Run()
- [ ] Clear watcher alongside FileTracker at iteration start
- [ ] Call MergeWatcherPaths before runAutoCommit
- [ ] Graceful fallback if watcher creation fails
- [ ] Stop watcher on shutdown

### 5. Fix iteration #0 auto-commit gap
- [ ] Add fileTracker.Clear() + auto-commit check to runIteration0()

### 6. Tests
- [ ] Unit test FileWatcher: exclusion filtering, debouncing, path normalization
- [ ] Unit test MergeWatcherPaths: no-overwrite semantics
- [ ] Integration test: file created via os.WriteFile detected by watcher
- [ ] Integration test: excluded dirs not tracked

## Out of Scope

- Watching files outside working directory
- Polling fallback for network filesystems
- Custom exclude patterns beyond data_dir (can add later)
- File content diffing from watcher events
- Tracking file deletions for auto-commit

## Open Questions

1. Should we add a `watch_exclude` list config for arbitrary patterns?
   - **Deferred**: Start with hard-coded .git/node_modules + configurable data_dir. Extend later if needed.
