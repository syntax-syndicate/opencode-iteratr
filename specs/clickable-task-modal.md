# Clickable Task Modal

Add mouse click support to task items in sidebar. Clicking a task opens a modal overlay showing full task details.

## Overview

Integrate bubblezone library to enable clickable regions on task items. When user clicks a task, display a centered modal overlay with complete task information including ID, content, status, priority, dependencies, and timestamps.

## User Story

**As a** developer monitoring agent progress  
**I want** to click on a task to see its full details  
**So that** I can quickly inspect task information without switching views

## Requirements

### Functional

1. **Clickable Task Items**
   - Each task in sidebar wrapped in bubblezone zone
   - Visual hover feedback (cursor change handled by terminal)
   - Left-click opens task detail modal

2. **Task Detail Modal**
   - Centered overlay on screen
   - Shows: ID, content (full text), status, priority, dependencies, created/updated timestamps
   - Dismissible via ESC, click outside, or close button zone

3. **Modal Behavior**
   - Blocks keyboard input to underlying components
   - Click outside modal dismisses it
   - Only one modal open at a time

### Non-Functional

1. No performance regression on task list rendering
2. Zone markers must not affect lipgloss width calculations
3. Modal must render correctly at all supported terminal sizes

## Technical Implementation

### Dependencies

Add bubblezone v2 to go.mod:
```
github.com/lrstanley/bubblezone/v2 (branch: v2-exp)
```

### Architecture

```
App.View()
├── zone.NewGlobal() called once at startup
├── Sidebar.Draw()
│   └── zone.Mark("task_{id}", taskContent) for each task
├── Modal overlay (when open)
│   └── zone.Mark("modal_box", modalContent)
└── zone.Scan(content) at root level
```

### Zone ID Convention

```go
// Task zones: "task_{xid}"
zoneID := fmt.Sprintf("task_%s", task.ID)

// Modal zone: "task_modal"
modalZoneID := "task_modal"
```

### Click Detection Flow

```
tea.MouseReleaseMsg
├── If modal open:
│   ├── Click inside modal → ignore (keep open)
│   └── Click outside modal → close modal
└── If modal closed:
    └── Check each task zone → open modal for clicked task
```

### Modal Component

```go
type TaskModal struct {
    task    *session.Task
    visible bool
    width   int
    height  int
}

func (m *TaskModal) Draw(scr uv.Screen, area uv.Rectangle) string {
    // Returns zone.Mark wrapped modal content
}
```

### App Integration

```go
type App struct {
    // ... existing fields
    taskModal *TaskModal
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.MouseReleaseMsg:
        if msg.Button != tea.MouseLeft {
            return a, nil
        }
        return a.handleTaskClick(msg)
    }
}

func (a *App) View() tea.View {
    // ... existing draw logic
    content := canvas.Render()
    
    // Overlay modal if visible
    if a.taskModal.visible {
        content = a.taskModal.Overlay(content, a.width, a.height)
    }
    
    // Scan zones at root
    view.SetContent(zone.Scan(content))
    return view
}
```

## Tasks

### 1. Add bubblezone dependency
- [ ] Run `go get github.com/lrstanley/bubblezone/v2@v2-exp`
- [ ] Initialize `zone.NewGlobal()` in app startup (main.go or app.go NewApp)

### 2. Create modal component
- [ ] Create `internal/tui/modal.go` with TaskModal struct
- [ ] Implement `Draw()` method rendering task details panel
- [ ] Implement `SetTask(task)` and `Close()` methods
- [ ] Implement `IsVisible()` getter

### 3. Wrap tasks in zones
- [ ] Import bubblezone in `internal/tui/sidebar.go`
- [ ] Modify `renderTask()` to wrap output with `zone.Mark("task_{id}", content)`
- [ ] Ensure zone markers don't break existing styling

### 4. Add zone scanning to app
- [ ] Import bubblezone in `internal/tui/app.go`
- [ ] Wrap final content with `zone.Scan()` in `View()` method

### 5. Handle mouse clicks in app
- [ ] Add `taskModal *TaskModal` field to App struct
- [ ] Instantiate TaskModal in `NewApp()`
- [ ] Add `tea.MouseReleaseMsg` case in `Update()`
- [ ] Implement `handleTaskClick()` to check zones and open modal

### 6. Implement modal overlay rendering
- [ ] Add modal overlay logic in `View()` after main content render
- [ ] Use `lipgloss.Place()` to center modal on screen
- [ ] Wrap modal content in `zone.Mark("task_modal", content)`

### 7. Handle modal dismissal
- [ ] Check click outside modal zone to dismiss
- [ ] Add ESC key handler when modal is open
- [ ] Block other keyboard input when modal visible

### 8. Style modal component
- [ ] Add modal styles to `internal/tui/styles.go` (border, background, title)
- [ ] Format task details: ID, status badge, priority, content, timestamps
- [ ] Add visual separator between detail sections

### 9. Add keyboard shortcut to open modal
- [ ] Add `Enter` key handler in sidebar to open modal for cursor-selected task
- [ ] Track cursor position in sidebar task list (j/k navigation)
- [ ] Highlight currently selected task row
- [ ] Ensure keyboard and mouse interactions work together

### 10. Test and polish
- [ ] Test click detection accuracy at various terminal sizes
- [ ] Test modal positioning on small terminals
- [ ] Verify no performance regression in task list rendering
- [ ] Test keyboard dismissal (ESC)
- [ ] Test keyboard open (Enter on selected task)

## UI Mockup

### Sidebar with Clickable Tasks
```
┌─ Tasks ─────────────────┐
│ ► Implement feature X   │  ← click opens modal
│ ○ Write tests           │
│ ○ Update docs           │
│ ✓ Setup project         │
└─────────────────────────┘
```

### Task Detail Modal
```
╭─ Task Details ──────────────────────────────────────╮
│                                                     │
│  ID: bq4567ab                                       │
│                                                     │
│  Status: ► in_progress    Priority: high           │
│                                                     │
│  ─────────────────────────────────────────────────  │
│                                                     │
│  Implement the new feature X that allows users to   │
│  track their usage metrics and export them to       │
│  various formats including CSV, JSON, and PDF.      │
│                                                     │
│  ─────────────────────────────────────────────────  │
│                                                     │
│  Depends on: cg7890cd, dh1234ef                     │
│                                                     │
│  Created:  2024-01-15 14:32:00                      │
│  Updated:  2024-01-15 16:45:30                      │
│                                                     │
│              [ESC or click outside to close]        │
╰─────────────────────────────────────────────────────╯
```

## Out of Scope

- Edit task from modal (view only)
- Task actions (mark complete, change status) from modal
- Multiple task selection
- Drag-and-drop task reordering
- Task context menu (right-click)
- Task history/changelog (not stored in current data model)
