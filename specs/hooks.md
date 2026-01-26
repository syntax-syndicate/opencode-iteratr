# Hooks

## Overview

Pre-iteration hooks allow users to run shell commands before each iteration, injecting dynamic context into the agent prompt.

## User Story

As a developer, I want to run custom scripts before each iteration so the agent receives up-to-date context (git status, test results, build output, etc.).

## Requirements

- Config file: `.iteratr.hooks.yml` in working directory
- Single hook type: `pre_iteration`
- Shell command execution with stdout capture
- Template variable expansion in commands (`{{iteration}}`, `{{session}}`)
- 30 second default timeout
- Graceful error handling (continue iteration with error in output)
- Raw output (no framing headers)

## Config Format

```yaml
version: 1

hooks:
  pre_iteration:
    command: "./scripts/context.sh {{iteration}}"
    timeout: 30  # optional, seconds
```

## Technical Implementation

### New Package: `internal/hooks/`

**types.go** - Configuration structs:
```go
type Config struct {
    Version int         `yaml:"version"`
    Hooks   HooksConfig `yaml:"hooks"`
}

type HooksConfig struct {
    PreIteration *HookConfig `yaml:"pre_iteration"`
}

type HookConfig struct {
    Command string `yaml:"command"`
    Timeout int    `yaml:"timeout"` // seconds, default 30
}
```

**hooks.go** - Loading and execution:
- `LoadConfig(workDir string) (*Config, error)` - Load `.iteratr.hooks.yml`, return nil if not found
- `Execute(ctx context.Context, hook *HookConfig, vars map[string]string) (string, error)` - Run command, expand vars, capture output

### Template Changes (`internal/template/template.go`)

1. Add `Hooks` field to `Variables` struct
2. Add `HookOutput` field to `BuildConfig` struct  
3. Add `{{hooks}}` to `replacements` map in `Render()`
4. Set `vars.Hooks = cfg.HookOutput` in `BuildPrompt()`

### Orchestrator Changes (`internal/orchestrator/orchestrator.go`)

1. Add `hooksConfig *hooks.Config` field to Orchestrator
2. Load hooks config in `Start()` (optional, log if missing)
3. Execute pre-iteration hook before `BuildPrompt()` call (between lines 347-350)
4. Pass hook output to `template.BuildConfig.HookOutput`

### Error Handling

- Config not found: Skip hooks, continue normally
- Config parse error: Log warning, continue without hooks
- Command failure/timeout: Include error in output, continue iteration
- Context cancelled: Propagate cancellation

### TUI Safety

- Never write hook stderr to `os.Stderr`
- Capture stderr, include in output or log via logger
- Use `cmd.Output()` or pipe-based capture

## Tasks

### 1. Add YAML dependency
- [ ] Add `gopkg.in/yaml.v3` to go.mod

### 2. Create hooks package
- [ ] Create `internal/hooks/types.go` with Config structs
- [ ] Create `internal/hooks/hooks.go` with LoadConfig and Execute functions

### 3. Extend template system
- [ ] Add `Hooks` field to Variables struct
- [ ] Add `HookOutput` field to BuildConfig struct
- [ ] Add `{{hooks}}` placeholder to Render() replacements
- [ ] Populate vars.Hooks in BuildPrompt()

### 4. Integrate into orchestrator
- [ ] Add hooksConfig field to Orchestrator struct
- [ ] Load hooks config in Start() with graceful fallback
- [ ] Execute pre-iteration hook in Run() before BuildPrompt
- [ ] Pass hook output to template.BuildConfig

### 5. Test manually
- [ ] Create test `.iteratr.hooks.yml` and verify hook execution

## Out of Scope

- Post-iteration hooks
- Multiple pre-iteration hooks
- Hook execution order/chaining
- Environment variable injection in config
- Hook-specific working directories

## Open Questions

None - all requirements clarified.
