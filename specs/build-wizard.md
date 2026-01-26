# Build Wizard

Interactive wizard for `iteratr build` when no spec file provided. Guides user through file selection, model choice, prompt editing, and session configuration.

## Overview

When `iteratr build` runs without `--spec` flag, launch a standalone TUI wizard before orchestrator initialization. Four steps: file picker, model selector, template editor, session config. Wizard returns selected values or cancels program.

## User Story

**As a** developer starting a new iteratr session  
**I want** a guided setup experience  
**So that** I can configure all session parameters without memorizing CLI flags

## Requirements

### Functional

1. **Trigger**: `iteratr build` without `--spec` flag (and not `--headless`)
2. **Step 1 - File Picker**: Browse cwd, filter `.md`/`.txt` files, select spec
3. **Step 2 - Model Selector**: Fetch models via `opencode models`, fuzzy filter list
4. **Step 3 - Template Editor**: Full textarea editing of built-in default template
5. **Step 4 - Config**: Session name (default: spec filename), iterations (default: 0)
6. **Navigation**: Back/Next buttons, Tab cycles fields, ESC goes back (step 1 confirms exit)
7. **Shortcuts**: Ctrl+Enter finishes wizard from any step (if all steps valid)
8. **Cancel**: Exit program with no action

### Non-Functional

1. Wizard runs as separate BubbleTea program before orchestrator exists
2. No flicker between wizard and main TUI
3. File picker handles large directories (lazy rendering)

## Technical Implementation

### Architecture

```
iteratr build (no --spec)
└── RunWizard() - standalone tea.Program
    ├── WizardModel
    │   ├── step int (0-3)
    │   ├── FilePickerStep
    │   │   └── ScrollList + os.ReadDir
    │   ├── ModelSelectorStep
    │   │   └── ScrollList + exec "opencode models"
    │   ├── TemplateEditorStep
    │   │   └── textarea.Model (Bubbles v2)
    │   └── ConfigStep
    │       ├── textinput (session name)
    │       └── textinput (iterations)
    └── Returns WizardResult or error
        └── Applied to buildFlags before orchestrator creation
```

### WizardResult

```go
type WizardResult struct {
    SpecPath    string
    Model       string
    Template    string  // Full edited template content
    SessionName string
    Iterations  int
}
```

### Build Integration

```go
func runBuild(cmd *cobra.Command, args []string) error {
    if buildFlags.spec == "" && !buildFlags.headless {
        result, err := tui.RunWizard()
        if err != nil {
            return err
        }
        buildFlags.spec = result.SpecPath
        buildFlags.model = result.Model
        // Template written to temp file or passed via new config field
        buildFlags.name = result.SessionName
        buildFlags.iterations = result.Iterations
    }
    // ... existing orchestrator creation
}
```

### File Picker Step

```go
type FilePickerStep struct {
    currentPath string
    items       []FileItem
    scrollList  *ScrollList
    selectedIdx int
}

type FileItem struct {
    name  string
    path  string
    isDir bool
}
```

**Behavior**:
- Root: cwd (current working directory)
- Filter: `.md` and `.txt` files only (show directories for navigation)
- Enter on directory: descend
- Backspace: go up one level
- Enter on file: select and advance

### Model Selector Step

```go
type ModelSelectorStep struct {
    allModels    []ModelInfo  // Full list from opencode
    filtered     []ModelInfo  // Filtered by search
    scrollList   *ScrollList
    selectedIdx  int
    searchInput  textinput.Model  // Fuzzy search input
    loading      bool
    error        string
}

type ModelInfo struct {
    id   string  // "anthropic/claude-sonnet-4-5"
    name string  // Display name
}
```

**Fuzzy filtering**: As user types in search input, filter `allModels` to `filtered` using case-insensitive substring match. Update ScrollList items on each keystroke.

**Fetch models**:
```go
func fetchModels() tea.Cmd {
    return func() tea.Msg {
        cmd := exec.Command("opencode", "models")
        output, err := cmd.Output()
        if err != nil {
            return ModelsErrorMsg{err}
        }
        // Output is newline-separated model IDs, one per line
        // e.g. "anthropic/claude-sonnet-4-5\nopenai/gpt-4o\n..."
        // First line is INFO log, skip lines starting with "INFO"
        models := parseModelsOutput(output)
        return ModelsLoadedMsg{models}
    }
}

func parseModelsOutput(output []byte) []ModelInfo {
    var models []ModelInfo
    for _, line := range strings.Split(string(output), "\n") {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "INFO") {
            continue
        }
        models = append(models, ModelInfo{id: line, name: line})
    }
    return models
}
```

### Template Editor Step

```go
type TemplateEditorStep struct {
    textarea textarea.Model
    content  string
}
```

**Behavior**:
- Pre-populated with `template.DefaultTemplate`
- Full editing capability
- Show placeholder variables reference at bottom

### Config Step

```go
type ConfigStep struct {
    sessionInput    textinput.Model
    iterationsInput textinput.Model
    focusIndex      int  // 0=session, 1=iterations
}
```

**Smart defaults**:
- Session name: spec filename stem, sanitized (alphanumeric, hyphens, underscores)
- Iterations: "0" (infinite)

### Navigation Messages

```go
type WizardNextMsg struct{}
type WizardBackMsg struct{}
type WizardCancelMsg struct{}
type WizardCompleteMsg struct{ Result WizardResult }
```

### Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| Tab | Any step | Cycle focus within step |
| Shift+Tab | Any step | Cycle focus backward |
| Enter | File/Model lists | Select item, advance |
| Enter | Config inputs | Advance to next (or finish on last) |
| Backspace | File picker (no input) | Go up directory |
| ESC | Step 2-4 | Go back one step |
| ESC | Step 1 | Confirm exit dialog |
| Ctrl+Enter | Any step | Finish wizard (if valid) |
| j/k or arrows | Lists | Navigate items |

### UI Layout

Each step uses same modal container with:
- Title bar showing step N of 4
- Content area for step component
- Footer with Back/Next buttons and hints

```
╭─ Build Wizard ─ Step 1 of 4 ────────────────────────╮
│                                                      │
│  Select Spec File                                    │
│  /home/user/project                                  │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ 📁 specs/                                      │  │
│  │ 📁 docs/                                       │  │
│  │ 📄 README.md                                   │  │
│  │ 📄 CONTRIBUTING.md                             │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│            [ Cancel ]              [ Next → ]        │
│                                                      │
│  ↑↓ navigate · enter select · backspace up           │
╰──────────────────────────────────────────────────────╯
```

## Tasks

### 1. Scaffold wizard infrastructure

- [ ] Create `internal/tui/wizard/wizard.go` with `WizardModel` struct
- [ ] Add `step int`, `cancelled bool`, `result WizardResult` fields
- [ ] Implement `Init()`, `Update()`, `View()` for BubbleTea
- [ ] Add `RunWizard() (*WizardResult, error)` entry point
- [ ] Wire into `cmd/iteratr/build.go` - call when `--spec` empty and not headless

### 2. File picker step

- [ ] Create `internal/tui/wizard/file_picker.go` with `FilePickerStep` struct
- [ ] Implement `loadDirectory(path)` using `os.ReadDir`
- [ ] Filter to `.md`, `.txt` files and directories
- [ ] Use `ScrollList` for item display with file/directory icons
- [ ] Handle Enter: directory descends, file selects
- [ ] Handle Backspace: go up one level (stop at root)
- [ ] Show current path in header

### 3. Model selector step

- [ ] Create `internal/tui/wizard/model_selector.go` with `ModelSelectorStep` struct
- [ ] Implement `fetchModels()` cmd - exec `opencode models`, parse output
- [ ] Show loading spinner while fetching
- [ ] Handle error state with retry option
- [ ] Add textinput for fuzzy search filter
- [ ] Implement fuzzy filter: case-insensitive substring match on model ID
- [ ] Use `ScrollList` for filtered model display, update on each keystroke
- [ ] Handle Enter: select model, advance

### 4. Template editor step

- [ ] Create `internal/tui/wizard/template_editor.go` with `TemplateEditorStep` struct
- [ ] Initialize textarea with `template.DefaultTemplate`
- [ ] Configure: word wrap, large height, no char limit
- [ ] Show placeholder variables reference below textarea
- [ ] Return edited content in result

### 5. Config step

- [ ] Create `internal/tui/wizard/config_step.go` with `ConfigStep` struct
- [ ] Add session name textinput with smart default from spec filename
- [ ] Add iterations textinput with default "0"
- [ ] Validate: session name non-empty, valid chars; iterations numeric >= 0
- [ ] Tab cycles between inputs
- [ ] Show validation errors inline

### 6. Navigation and buttons

- [ ] Add button bar component with Back/Next/Cancel/Finish states
- [ ] Implement Tab cycling within each step
- [ ] Implement ESC to go back (step 1 shows confirm dialog)
- [ ] Implement Ctrl+Enter to finish from any step
- [ ] Disable Next when step invalid (no selection, empty required field)

### 7. Step container and styling

- [ ] Create shared step container with title, step indicator, content area, buttons
- [ ] Apply modal styling consistent with `NoteInputModal`
- [ ] Add hint bar at bottom with context-sensitive keybindings
- [ ] Handle terminal resize

### 8. Integration and edge cases

- [ ] Write edited template to temp file, pass path to orchestrator
- [ ] Handle `opencode` not installed (show error, allow manual model entry)
- [ ] Handle empty directories in file picker
- [ ] Handle very long file/model lists (ScrollList lazy render)
- [ ] Clean up temp template file after session ends

## UI Mockups

### Step 1: File Picker
```
╭─ Build Wizard ─ Step 1 of 4 ────────────────────────╮
│                                                      │
│  Select Spec File                                    │
│  ~/project/specs                                     │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ 📁 ..                                          │  │
│  │ 📄 build-wizard.md                      ←      │  │
│  │ 📄 user-note-creation.md                       │  │
│  │ 📄 tui-beautification.md                       │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│            [ Cancel ]              [ Next → ]        │
│                                                      │
│  ↑↓ navigate · enter select · backspace up           │
╰──────────────────────────────────────────────────────╯
```

### Step 2: Model Selector
```
╭─ Build Wizard ─ Step 2 of 4 ────────────────────────╮
│                                                      │
│  Select Model                                        │
│                                                      │
│  Search: claude-son█                                 │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ anthropic/claude-sonnet-4-5              ←     │  │
│  │ anthropic/claude-sonnet-4-0                    │  │
│  │ github-copilot/claude-sonnet-4.5               │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│        [ ← Back ]                  [ Next → ]        │
│                                                      │
│  type to filter · ↑↓ navigate · enter select         │
╰──────────────────────────────────────────────────────╯
```

### Step 3: Template Editor
```
╭─ Build Wizard ─ Step 3 of 4 ────────────────────────╮
│                                                      │
│  Edit Prompt Template                                │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │ # iteratr Session                              │  │
│  │ Session: {{session}} | Iteration: #{{iteration│  │
│  │                                                │  │
│  │ {{history}}                                    │  │
│  │ ## Spec                                        │  │
│  │ {{spec}}                                       │  │
│  │ ...                                            │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  Variables: {{session}} {{iteration}} {{spec}}       │
│             {{notes}} {{tasks}} {{history}} {{extra}}│
│                                                      │
│        [ ← Back ]                  [ Next → ]        │
│                                                      │
│  ctrl+enter finish · esc back                        │
╰──────────────────────────────────────────────────────╯
```

### Step 4: Session Config
```
╭─ Build Wizard ─ Step 4 of 4 ────────────────────────╮
│                                                      │
│  Session Configuration                               │
│                                                      │
│  Session Name                                        │
│  ┌────────────────────────────────────────────────┐  │
│  │ build-wizard                                   │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  Max Iterations (0 = infinite)                       │
│  ┌────────────────────────────────────────────────┐  │
│  │ 0                                              │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│        [ ← Back ]                  [ Finish ]        │
│                                                      │
│  tab cycle · ctrl+enter finish · esc back            │
╰──────────────────────────────────────────────────────╯
```

## Gotchas

### 1. Separate BubbleTea program

Wizard runs BEFORE orchestrator exists. Must be a standalone `tea.NewProgram()` that blocks until complete. Cannot share state with main TUI - only returns result struct.

### 2. opencode models output format

Output is newline-separated model IDs (e.g. `anthropic/claude-sonnet-4-5`). First line may be INFO log - filter lines starting with "INFO". No custom model entry - user must pick from list.

### 3. Template temp file lifecycle

Edited template saved to temp file. Must ensure:
- File created with secure permissions
- Path passed to orchestrator config (new field or modified TemplatePath)
- Cleanup on session end (even on crash)

### 4. Session name validation

Must match existing validation in `build.go:79-90`:
- Non-empty
- Max 64 chars
- Alphanumeric, hyphens, underscores only

### 5. File picker root boundary

User should be able to navigate anywhere from cwd, but consider adding safeguard against navigating into system directories or above home directory.

## Out of Scope

- Saving wizard presets/profiles
- Recent files list
- Model favorites
- Template library/selection
- Drag-and-drop file selection
- Custom keybinding configuration
- Wizard skip flag (use `--spec` instead)
- Custom model ID entry (must pick from opencode list)
- Persisting edited template to disk (session-only)

## Open Questions

None - all resolved.
