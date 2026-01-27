# Test Spec

## Overview

E2E test spec for validating iteratr's TUI and task workflow. Creates, edits, and deletes temporary files to exercise the full pipeline without leaving artifacts.

## User Story

As a developer, I want to verify iteratr works end-to-end so I can catch regressions.

## Requirements

- DO NOT COMMIT ANYTHING
- All created files MUST be cleaned up before session ends
- Exercise file creation, editing, reading, and deletion
- Validate task state transitions work correctly
- All temp files go in `/tmp/iteratr-test/` or use `.test-` prefix in repo root

## Tasks

### 1. Research (use subagents for parallel work)
- [ ] Use a subagent to find all Go files in `internal/tui/` and list their main types
- [ ] Use a subagent to analyze `internal/session/store.go` and summarize its public API

### 2. File Creation
- [ ] Create directory `/tmp/iteratr-test/`
- [ ] Create a Go file `/tmp/iteratr-test/hello.go` with a simple main package
- [ ] Create a JSON config file `/tmp/iteratr-test/config.json` with test data
- [ ] Create a markdown file `/tmp/iteratr-test/notes.md` with placeholder content
- [ ] Write research findings to `/tmp/iteratr-test/research.md`

### 3. File Editing
- [ ] Edit `/tmp/iteratr-test/hello.go` - add a helper function
- [ ] Edit `/tmp/iteratr-test/config.json` - add new key-value pairs
- [ ] Edit `/tmp/iteratr-test/notes.md` - append a new section

### 4. File Validation
- [ ] Read each file and verify edits are present
- [ ] List all files in `/tmp/iteratr-test/` and confirm count matches expected (4 files)

### 5. Cleanup
- [ ] Delete all files in `/tmp/iteratr-test/`
- [ ] Remove the `/tmp/iteratr-test/` directory
- [ ] Verify no test artifacts remain

## UI Mockup

N/A - exercises backend task workflow

## Out of Scope

- Git commits
- Modifications to source code
- Network operations

## Open Questions

None
