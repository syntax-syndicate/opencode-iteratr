# TUI Beautification

Modernize iteratr's TUI with Crush-inspired patterns: Ultraviolet Screen/Draw rendering, rectangle-based layouts, component interfaces, centralized state, and consistent styling.

## Overview

Refactor TUI architecture to adopt production-proven patterns from Charmbracelet's Crush. Replace string-based `View()` rendering with Ultraviolet's `Screen/Draw` pattern for pixel-perfect layout control. Introduce formal component interfaces, centralize state management, and standardize on Bubbles v2 viewport for all scrollable areas.

## User Story

**As a** developer using iteratr  
**I want** a polished, responsive TUI that adapts to terminal size  
**So that** I can monitor agent progress efficiently regardless of my terminal dimensions

## Requirements

### Functional

1. **Screen/Draw Rendering Pattern**
   - Replace `View() string` with `Draw(scr uv.Screen, area uv.Rectangle)`
   - Components render directly to screen buffer at precise coordinates
   - Eliminates string concatenation and manual positioning

2. **Ultraviolet Layout System**
   - Replace manual dimension calculations with `uv.SplitVertical`/`uv.SplitHorizontal`
   - Centralized layout struct defining all UI regions
   - Dynamic sizing propagated to child components via rectangles

3. **Responsive Breakpoints**
   - Compact mode when width < 100 or height < 25
   - Desktop mode: sidebar + main content side-by-side
   - Compact mode: stacked layout, collapsible sidebar

4. **Component Interface System**
   - Formal `Drawable`, `Sizable`, `Focusable`, `Stateful` interfaces
   - All views implement consistent contracts
   - Enable generic component handling and composition

5. **Centralized State Management**
   - Single `*session.State` in App, no duplication
   - Components receive read-only state views
   - Eliminates state sync bugs

6. **Bubbles Viewport Standardization**
   - Replace manual scroll logic in LogViewer, TaskSidebar
   - Consistent scroll behavior across all scrollable components
   - Lazy rendering for performance

7. **Hierarchical Keyboard Routing**
   - Global → State → Focus → Component priority
   - Clean delegation without code duplication
   - Modal/overlay support foundation

8. **Enhanced Visual Polish**
   - Refreshed color palette (modern, cohesive scheme)
   - Consistent border styles and spacing
   - Focus indicators with visual distinction
   - Status bar with dynamic content

9. **Animation Support**
   - Spinner animation for "working" state
   - Pulse animation for new messages/updates
   - Smooth visual feedback on state changes

10. **Sidebar Content**
    - Tasks section: grouped by status (in_progress → remaining → completed)
    - Notes section: recent notes with type indicators
    - Scroll indicators when content overflows

### Non-Functional

1. No breaking changes to NATS data structures
2. Maintain current functionality during refactor
3. Performance: No perceptible lag on resize
4. Memory: No increase in baseline usage

## Technical Implementation

### Architecture Change

**Before:**
```
App.View() string
├── header := renderHeader()           # String concatenation
├── content := activeView.Render()     # More strings
├── footer := renderFooter()           # Even more strings
└── return lipgloss.JoinVertical(...)  # Manual assembly
```

**After:**
```
App.View() tea.View
├── canvas := uv.NewScreenBuffer(width, height)
├── cursor := a.Draw(canvas, canvas.Bounds())
│   ├── a.header.Draw(canvas, layout.Header)
│   ├── a.drawActiveView(canvas, layout.Main)
│   ├── a.sidebar.Draw(canvas, layout.Sidebar)
│   ├── a.status.Draw(canvas, layout.Status)
│   └── a.footer.Draw(canvas, layout.Footer)
├── view.Content = canvas.Render()
├── view.Cursor = cursor
└── return view
```

### Package Structure

```
internal/tui/
├── interfaces.go     # NEW: Drawable, Sizable, Focusable, Stateful
├── layout.go         # NEW: Ultraviolet layout management
├── draw.go           # NEW: Common draw helpers (borders, text, panels)
├── anim.go           # NEW: Animation utilities (spinner, pulse)
├── app.go            # MODIFY: Screen/Draw pattern, centralized state
├── styles.go         # MODIFY: Refreshed color palette, theme-ready system
├── header.go         # NEW: Header component with Draw method
├── footer.go         # NEW: Footer component with Draw method
├── status.go         # NEW: Status bar component with Draw method
├── dashboard.go      # MODIFY: Implement Drawable interface
├── logs.go           # MODIFY: Use viewport, implement Drawable
├── notes.go          # MODIFY: Implement Drawable
├── inbox.go          # MODIFY: Implement Drawable
├── sidebar.go        # MODIFY: Rename TaskSidebar→Sidebar, add Notes section, use viewport
└── agent.go          # MODIFY: Implement Drawable (already uses viewport)
```

### Component Interfaces

```go
// internal/tui/interfaces.go
package tui

import (
    tea "charm.land/bubbletea/v2"
    uv "github.com/charmbracelet/ultraviolet"
    "github.com/mark3labs/iteratr/internal/session"
)

// Drawable components render to a screen rectangle
type Drawable interface {
    Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor
}

// Updateable components handle messages
type Updateable interface {
    Update(tea.Msg) tea.Cmd
}

// Sizable components track their dimensions
type Sizable interface {
    SetSize(width, height int)
}

// Focusable components track focus state
type Focusable interface {
    SetFocus(focused bool)
    IsFocused() bool
}

// Stateful components receive state updates
type Stateful interface {
    SetState(state *session.State)
}

// Component combines Drawable and Updateable
type Component interface {
    Drawable
    Updateable
}

// FullComponent combines all standard interfaces
type FullComponent interface {
    Component
    Sizable
    Stateful
}

// FocusableComponent adds focus to FullComponent
type FocusableComponent interface {
    FullComponent
    Focusable
}
```

### Layout System

```go
// internal/tui/layout.go
package tui

import uv "github.com/charmbracelet/ultraviolet"

const (
    CompactWidthBreakpoint  = 100
    CompactHeightBreakpoint = 25
    SidebarWidthDesktop     = 45
    HeaderHeight            = 1
    StatusHeight            = 1
    FooterHeight            = 1
)

type LayoutMode int
const (
    LayoutDesktop LayoutMode = iota
    LayoutCompact
)

type Layout struct {
    Mode      LayoutMode
    Area      uv.Rectangle
    Header    uv.Rectangle
    Content   uv.Rectangle
    Main      uv.Rectangle
    Sidebar   uv.Rectangle
    Status    uv.Rectangle
    Footer    uv.Rectangle
}

func CalculateLayout(width, height int) Layout {
    mode := LayoutDesktop
    if width < CompactWidthBreakpoint || height < CompactHeightBreakpoint {
        mode = LayoutCompact
    }

    area := uv.NewRect(0, 0, width, height)
    
    // Vertical splits: header | content | status | footer
    rows := uv.SplitVertical(area,
        uv.Fixed(HeaderHeight),   // header
        uv.Flex(1),               // content (main + sidebar)
        uv.Fixed(StatusHeight),   // status
        uv.Fixed(FooterHeight),   // footer
    )
    
    headerRect := rows[0]
    contentRect := rows[1]
    statusRect := rows[2]
    footerRect := rows[3]
    
    var mainRect, sidebarRect uv.Rectangle
    if mode == LayoutDesktop {
        sidebarWidth := min(SidebarWidthDesktop, contentRect.Dx()/3)
        cols := uv.SplitHorizontal(contentRect,
            uv.Flex(1),              // main
            uv.Fixed(sidebarWidth),  // sidebar
        )
        mainRect = cols[0]
        sidebarRect = cols[1]
    } else {
        mainRect = contentRect
        sidebarRect = uv.Rectangle{} // Hidden
    }

    return Layout{
        Mode:    mode,
        Area:    area,
        Header:  headerRect,
        Content: contentRect,
        Main:    mainRect,
        Sidebar: sidebarRect,
        Status:  statusRect,
        Footer:  footerRect,
    }
}
```

### Draw Helpers

```go
// internal/tui/draw.go
package tui

import (
    uv "github.com/charmbracelet/ultraviolet"
    "charm.land/lipgloss/v2"
)

// DrawText renders styled text at a position
func DrawText(scr uv.Screen, area uv.Rectangle, text string) {
    uv.NewStyledString(text).Draw(scr, area)
}

// DrawStyled renders lipgloss-styled content
func DrawStyled(scr uv.Screen, area uv.Rectangle, style lipgloss.Style, text string) {
    content := style.MaxWidth(area.Dx()).MaxHeight(area.Dy()).Render(text)
    uv.NewStyledString(content).Draw(scr, area)
}

// DrawBorder renders a border around an area, returns inner area
func DrawBorder(scr uv.Screen, area uv.Rectangle, style lipgloss.Style) uv.Rectangle {
    // Render border frame
    border := style.Width(area.Dx()).Height(area.Dy()).Render("")
    uv.NewStyledString(border).Draw(scr, area)
    
    // Return inner content area (accounting for border)
    return uv.NewRect(
        area.Min.X+1,
        area.Min.Y+1,
        area.Dx()-2,
        area.Dy()-2,
    )
}

// DrawPanel renders a bordered panel with optional title
func DrawPanel(scr uv.Screen, area uv.Rectangle, title string, focused bool) uv.Rectangle {
    style := borderStyle(focused)
    inner := DrawBorder(scr, area, style)
    
    if title != "" {
        titleStyle := stylePanelTitle
        if focused {
            titleStyle = stylePanelTitleFocused
        }
        titleText := titleStyle.Render(" " + title + " ")
        // Draw title at top-left of border
        titleArea := uv.NewRect(area.Min.X+2, area.Min.Y, len(title)+2, 1)
        uv.NewStyledString(titleText).Draw(scr, titleArea)
    }
    
    return inner
}

// FillArea clears an area with a style
func FillArea(scr uv.Screen, area uv.Rectangle, style lipgloss.Style) {
    fill := style.Width(area.Dx()).Height(area.Dy()).Render("")
    uv.NewStyledString(fill).Draw(scr, area)
}
```

### App with Screen/Draw

```go
// internal/tui/app.go
type App struct {
    // State - single source of truth
    state       *session.State
    
    // Layout
    width, height int
    layout        Layout
    
    // Components
    header    *Header
    dashboard *Dashboard
    logs      *LogViewer
    notes     *NotesPanel
    inbox     *InboxPanel
    sidebar   *Sidebar  // Contains Tasks + Notes sections
    status    *StatusBar
    footer    *Footer
    agent     *AgentOutput
    
    // View state
    activeView ViewType
    focus      FocusArea
}

// View returns tea.View struct (Bubbletea v2 pattern)
func (a *App) View() tea.View {
    var view tea.View
    
    // Configure view properties
    view.AltScreen = true
    view.MouseMode = tea.MouseModeCellMotion
    
    // Recalculate layout if needed
    if a.layoutDirty {
        a.layout = CalculateLayout(a.width, a.height)
        a.propagateSizes()
        a.layoutDirty = false
    }
    
    // Create screen buffer for drawing
    canvas := uv.NewScreenBuffer(a.width, a.height)
    
    // Draw all components to canvas
    view.Cursor = a.Draw(canvas, canvas.Bounds())
    
    // Convert canvas to string content
    view.Content = canvas.Render()
    
    return view
}

// Draw renders all components to the screen buffer
func (a *App) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
    var cursor *tea.Cursor
    
    // Draw all regions
    a.header.Draw(scr, a.layout.Header)
    cursor = a.drawActiveView(scr, a.layout.Main)
    
    if a.layout.Mode == LayoutDesktop {
        a.sidebar.Draw(scr, a.layout.Sidebar)
    }
    
    a.status.Draw(scr, a.layout.Status)
    a.footer.Draw(scr, a.layout.Footer)
    
    return cursor
}

func (a *App) drawActiveView(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
    switch a.activeView {
    case ViewDashboard:
        return a.dashboard.Draw(scr, area)
    case ViewLogs:
        return a.logs.Draw(scr, area)
    case ViewNotes:
        return a.notes.Draw(scr, area)
    case ViewInbox:
        return a.inbox.Draw(scr, area)
    }
    return nil
}

func (a *App) propagateSizes() {
    a.header.SetSize(a.layout.Header.Dx(), a.layout.Header.Dy())
    a.dashboard.SetSize(a.layout.Main.Dx(), a.layout.Main.Dy())
    a.logs.SetSize(a.layout.Main.Dx(), a.layout.Main.Dy())
    a.notes.SetSize(a.layout.Main.Dx(), a.layout.Main.Dy())
    a.inbox.SetSize(a.layout.Main.Dx(), a.layout.Main.Dy())
    a.status.SetSize(a.layout.Status.Dx(), a.layout.Status.Dy())
    a.footer.SetSize(a.layout.Footer.Dx(), a.layout.Footer.Dy())
    
    if a.layout.Mode == LayoutDesktop {
        a.sidebar.SetSize(a.layout.Sidebar.Dx(), a.layout.Sidebar.Dy())
    }
}
```

### Component with Draw (Dashboard Example)

```go
// internal/tui/dashboard.go
type Dashboard struct {
    agent   *AgentOutput
    width   int
    height  int
    state   *session.State
    focused bool
}

func (d *Dashboard) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
    // Draw panel border
    inner := DrawPanel(scr, area, "Agent Output", d.focused)
    
    // Draw agent output in inner area
    return d.agent.Draw(scr, inner)
}

func (d *Dashboard) SetSize(width, height int) {
    d.width, d.height = width, height
    // Account for border when setting agent size
    d.agent.SetSize(width-2, height-2)
}

func (d *Dashboard) SetState(state *session.State) {
    d.state = state
}

func (d *Dashboard) SetFocus(focused bool) {
    d.focused = focused
}

func (d *Dashboard) IsFocused() bool {
    return d.focused
}

func (d *Dashboard) Update(msg tea.Msg) tea.Cmd {
    return d.agent.Update(msg)
}
```

### LogViewer with Viewport and Draw

```go
// internal/tui/logs.go
type LogViewer struct {
    viewport viewport.Model
    state    *session.State
    width    int
    height   int
    focused  bool
}

func NewLogViewer() *LogViewer {
    vp := viewport.New()
    return &LogViewer{viewport: vp}
}

func (l *LogViewer) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
    // Draw panel border
    inner := DrawPanel(scr, area, "Event Log", l.focused)
    
    // Draw viewport content
    content := l.viewport.View()
    DrawText(scr, inner, content)
    
    // Draw scroll indicator if needed
    if l.viewport.TotalLineCount() > l.viewport.Height {
        l.drawScrollIndicator(scr, area)
    }
    
    return nil
}

func (l *LogViewer) drawScrollIndicator(scr uv.Screen, area uv.Rectangle) {
    pct := l.viewport.ScrollPercent()
    indicator := fmt.Sprintf(" %d%% ", int(pct*100))
    indicatorArea := uv.NewRect(
        area.Max.X-len(indicator)-1,
        area.Max.Y-1,
        len(indicator),
        1,
    )
    DrawStyled(scr, indicatorArea, styleScrollIndicator, indicator)
}

func (l *LogViewer) SetSize(width, height int) {
    l.width, l.height = width, height
    l.viewport.SetWidth(width - 2)  // Account for border
    l.viewport.SetHeight(height - 2)
    l.updateContent()
}

func (l *LogViewer) SetState(state *session.State) {
    l.state = state
    l.updateContent()
}

func (l *LogViewer) SetFocus(focused bool) { l.focused = focused }
func (l *LogViewer) IsFocused() bool       { return l.focused }

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
    var cmd tea.Cmd
    l.viewport, cmd = l.viewport.Update(msg)
    return cmd
}

func (l *LogViewer) updateContent() {
    if l.state == nil {
        return
    }
    var b strings.Builder
    for _, event := range l.state.Events {
        b.WriteString(l.renderEvent(event))
        b.WriteString("\n")
    }
    l.viewport.SetContent(b.String())
}
```

### Hierarchical Keyboard Routing

```go
// internal/tui/app.go (keyboard handling)
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        // 1. Global keys (highest priority)
        if cmd := a.handleGlobalKeys(msg); cmd != nil {
            return a, cmd
        }
        
        // 2. View-level keys (switching views)
        if cmd := a.handleViewKeys(msg); cmd != nil {
            return a, cmd
        }
        
        // 3. Focus-specific keys
        if cmd := a.handleFocusKeys(msg); cmd != nil {
            return a, cmd
        }
        
        // 4. Delegate to active component
        return a, a.delegateToActive(msg)
        
    case tea.WindowSizeMsg:
        a.width, a.height = msg.Width, msg.Height
        a.layoutDirty = true
        return a, nil
    }
    // ... other message handling
}

func (a *App) handleGlobalKeys(msg tea.KeyPressMsg) tea.Cmd {
    switch msg.String() {
    case "q", "ctrl+c":
        return tea.Quit
    case "?":
        return a.toggleHelp()
    }
    return nil
}

func (a *App) handleViewKeys(msg tea.KeyPressMsg) tea.Cmd {
    switch msg.String() {
    case "1":
        a.activeView = ViewDashboard
        return nil
    case "2":
        a.activeView = ViewLogs
        return nil
    case "3":
        a.activeView = ViewNotes
        return nil
    case "4":
        a.activeView = ViewInbox
        return nil
    case "tab":
        a.cycleFocus()
        return nil
    }
    return nil
}
```

## Tasks

### Phase 0: Tasks List

#### 0.1 SYNC ALL TASKS FROM BELOW BEFORE DOING ANYTHING ELSE!!!! ALL OF THEM!!!

### Phase 1: Foundation

#### 1.1 Create Drawable Interface
- [ ] Create `internal/tui/interfaces.go`
- [ ] Define `Drawable` interface with `Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor`
- [ ] Define `Updateable` interface with `Update(tea.Msg) tea.Cmd`
- [ ] Define `Component` combining Drawable + Updateable

#### 1.2 Add Supporting Interfaces
- [ ] Define `Sizable` interface with `SetSize(width, height int)`
- [ ] Define `Focusable` interface with `SetFocus(bool)` and `IsFocused() bool`
- [ ] Define `Stateful` interface with `SetState(*session.State)`
- [ ] Define `FullComponent` and `FocusableComponent` composite interfaces

#### 1.3 Create Layout Constants
- [ ] Create `internal/tui/layout.go`
- [ ] Define `CompactWidthBreakpoint = 100`
- [ ] Define `CompactHeightBreakpoint = 25`
- [ ] Define `SidebarWidthDesktop = 45`
- [ ] Define `HeaderHeight`, `StatusHeight`, `FooterHeight` constants

#### 1.4 Create Layout Types
- [ ] Define `LayoutMode` enum (LayoutDesktop, LayoutCompact)
- [ ] Define `Layout` struct with all rectangle fields
- [ ] Add `IsCompact()` helper method to Layout

#### 1.5 Implement CalculateLayout Function
- [ ] Implement mode detection based on breakpoints
- [ ] Implement vertical splits for header/content/status/footer
- [ ] Implement horizontal split for main/sidebar in desktop mode
- [ ] Handle compact mode (no sidebar split)

### Phase 2: Draw Helpers

#### 2.1 Create Draw Helper File
- [ ] Create `internal/tui/draw.go`
- [ ] Import ultraviolet and lipgloss

#### 2.2 Implement Basic Draw Helpers
- [ ] Implement `DrawText(scr, area, text string)`
- [ ] Implement `DrawStyled(scr, area, style, text)`
- [ ] Implement `FillArea(scr, area, style)`

#### 2.3 Implement Panel Draw Helpers
- [ ] Implement `DrawBorder(scr, area, style) uv.Rectangle` returning inner area
- [ ] Implement `DrawPanel(scr, area, title, focused) uv.Rectangle`
- [ ] Implement `DrawScrollIndicator(scr, area, percent float64)`

#### 2.4 Implement Layout Draw Helpers
- [ ] Implement `DrawHorizontalDivider(scr, area, style)`
- [ ] Implement `DrawVerticalDivider(scr, area, style)`

### Phase 3: Header Component

#### 3.1 Create Header Component
- [ ] Create `internal/tui/header.go`
- [ ] Define `Header` struct with width, state fields
- [ ] Implement constructor `NewHeader()`

#### 3.2 Implement Header Interfaces
- [ ] Implement `Draw(scr, area)` - render app name, session, iteration, status
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetState(state)`
- [ ] Implement `Update(msg)` (minimal, mostly static)

### Phase 4: Footer Component

#### 4.1 Create Footer Component
- [ ] Create `internal/tui/footer.go`
- [ ] Define `Footer` struct with width, activeView, layoutMode fields
- [ ] Implement constructor `NewFooter()`

#### 4.2 Implement Footer Interfaces
- [ ] Implement `Draw(scr, area)` - render keybinding hints
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetActiveView(view ViewType)`
- [ ] Implement `SetLayoutMode(mode LayoutMode)`

#### 4.3 Footer Content Logic
- [ ] Render view shortcuts [1-4]
- [ ] Render sidebar toggle hint in compact mode
- [ ] Render help and quit hints
- [ ] Adapt content based on available width

### Phase 5: Status Bar Component

#### 5.1 Create Status Bar Component
- [ ] Create `internal/tui/status.go`
- [ ] Define `StatusBar` struct with width, state, status fields
- [ ] Implement constructor `NewStatusBar()`

#### 5.2 Implement Status Bar Interfaces
- [ ] Implement `Draw(scr, area)` - render connection status, current task
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetState(state)`
- [ ] Implement `SetConnectionStatus(connected bool)`

#### 5.3 Status Indicators
- [ ] Implement working indicator (◐)
- [ ] Implement connected indicator (●)
- [ ] Implement disconnected indicator (○)
- [ ] Implement error indicator (✗)

### Phase 6: Migrate AgentOutput to Draw

#### 6.1 Update AgentOutput Interface
- [ ] Add `Draw(scr, area) *tea.Cursor` method to AgentOutput
- [ ] Keep existing viewport-based rendering logic
- [ ] Return cursor position if text input is active

#### 6.2 Implement AgentOutput Draw
- [ ] Render viewport content to screen area
- [ ] Handle auto-scroll state
- [ ] Draw scroll position indicator

### Phase 7: Migrate Dashboard to Draw

#### 7.1 Update Dashboard Structure
- [ ] Remove `Render() string` method
- [ ] Add `Draw(scr, area) *tea.Cursor` method
- [ ] Update to use `DrawPanel` helper

#### 7.2 Implement Dashboard Draw
- [ ] Draw panel border with title
- [ ] Delegate to AgentOutput.Draw for content
- [ ] Pass through cursor from AgentOutput

#### 7.3 Update Dashboard Interfaces
- [ ] Implement `SetSize(width, height)` accounting for border
- [ ] Implement `SetState(state)`
- [ ] Implement `SetFocus(focused)` and `IsFocused()`
- [ ] Add compile-time interface check

### Phase 8: Migrate LogViewer to Viewport + Draw

#### 8.1 Replace Manual Scroll with Viewport
- [ ] Import `bubbles/v2/viewport` in logs.go
- [ ] Replace `offset int` field with `viewport viewport.Model`
- [ ] Remove `scrollUp`, `scrollDown`, `clampOffset` methods

#### 8.2 Implement LogViewer Draw
- [ ] Add `Draw(scr, area) *tea.Cursor` method
- [ ] Use `DrawPanel` for border
- [ ] Render viewport.View() content to inner area
- [ ] Draw scroll indicator when content overflows

#### 8.3 Update LogViewer State Management
- [ ] Implement `SetState(state)` to update content
- [ ] Implement `updateContent()` to rebuild viewport content
- [ ] Implement `SetSize(width, height)` for viewport dimensions

#### 8.4 Update LogViewer Interfaces
- [ ] Remove `Render() string` method
- [ ] Implement `SetFocus(focused)` and `IsFocused()`
- [ ] Update `Update(msg)` to delegate to viewport
- [ ] Add compile-time interface check

### Phase 9: Migrate NotesPanel to Draw

#### 9.1 Update NotesPanel Structure
- [ ] Remove `Render() string` method
- [ ] Add viewport for scrollable content
- [ ] Add focused state tracking

#### 9.2 Implement NotesPanel Draw
- [ ] Add `Draw(scr, area) *tea.Cursor` method
- [ ] Use `DrawPanel` for border with "Notes" title
- [ ] Render notes grouped by type
- [ ] Draw scroll indicator

#### 9.3 Update NotesPanel Interfaces
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetState(state)`
- [ ] Implement `SetFocus(focused)` and `IsFocused()`
- [ ] Add compile-time interface check

### Phase 10: Migrate InboxPanel to Draw

#### 10.1 Update InboxPanel Structure
- [ ] Remove `Render() string` method
- [ ] Add viewport for scrollable content
- [ ] Add focused state tracking

#### 10.2 Implement InboxPanel Draw
- [ ] Add `Draw(scr, area) *tea.Cursor` method
- [ ] Use `DrawPanel` for border with "Inbox" title
- [ ] Render messages with read/unread styling
- [ ] Draw scroll indicator

#### 10.3 Update InboxPanel Interfaces
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetState(state)`
- [ ] Implement `SetFocus(focused)` and `IsFocused()`
- [ ] Add compile-time interface check

### Phase 11: Migrate Sidebar to Viewport + Draw (Tasks + Notes)

#### 11.1 Rename and Restructure Sidebar
- [ ] Rename `TaskSidebar` to `Sidebar` (contains Tasks + Notes)
- [ ] Add `notesViewport viewport.Model` for notes section
- [ ] Add `tasksViewport viewport.Model` for tasks section
- [ ] Remove manual scroll logic

#### 11.2 Implement Sidebar Layout
- [ ] Split sidebar area vertically: Tasks (60%) | Notes (40%)
- [ ] Use `uv.SplitVertical` for dynamic sizing
- [ ] Add section headers ("Tasks", "Notes")

#### 11.3 Implement Sidebar Draw
- [ ] Add `Draw(scr, area) *tea.Cursor` method
- [ ] Draw tasks section with status grouping (in_progress → remaining → completed)
- [ ] Draw notes section with type indicators (tip, decision, learning, stuck)
- [ ] Draw scroll indicators for each section

#### 11.4 Add ID-Based Lookups
- [ ] Add `taskIndex map[string]int` field
- [ ] Add `noteIndex map[string]int` field
- [ ] Implement `rebuildIndex()` called on state update
- [ ] Replace linear searches with O(1) lookups

#### 11.5 Update Sidebar Interfaces
- [ ] Remove `Render() string` method
- [ ] Implement `SetSize(width, height)`
- [ ] Implement `SetState(state)`
- [ ] Keep existing `SetFocus(focused)` and `IsFocused()`
- [ ] Add compile-time interface check

### Phase 12: Refactor App to Screen/Draw

#### 12.1 Add Layout Management to App
- [ ] Add `layout Layout` field to App
- [ ] Add `layoutDirty bool` field for lazy recalculation
- [ ] Implement `propagateSizes()` method

#### 12.2 Centralize State in App
- [ ] Remove `state` field from Dashboard, LogViewer, NotesPanel, InboxPanel, TaskSidebar
- [ ] Add single `state *session.State` field to App
- [ ] Update state propagation to call `SetState()` on all components

#### 12.3 Create New Components in App
- [ ] Instantiate Header in NewApp
- [ ] Instantiate Footer in NewApp
- [ ] Instantiate StatusBar in NewApp

#### 12.4 Implement App.View() with Draw
- [ ] Change `View() string` to `View() tea.View`
- [ ] Return draw function that calls component Draw methods
- [ ] Calculate layout in draw function
- [ ] Draw header, active view, sidebar (if desktop), status, footer

#### 12.5 Remove Old View Logic
- [ ] Remove `renderHeader()` string method
- [ ] Remove `renderFooter()` string method
- [ ] Remove `renderContent()` string method
- [ ] Remove lipgloss.JoinVertical assembly

### Phase 13: Hierarchical Keyboard Routing

#### 13.1 Extract Key Handlers
- [ ] Create `handleGlobalKeys(msg) tea.Cmd` method
- [ ] Create `handleViewKeys(msg) tea.Cmd` method
- [ ] Create `handleFocusKeys(msg) tea.Cmd` method
- [ ] Create `delegateToActive(msg) tea.Cmd` method

#### 13.2 Implement Priority Routing
- [ ] Update `Update()` to call handlers in priority order
- [ ] Global keys: q, ctrl+c, ?
- [ ] View keys: 1, 2, 3, 4
- [ ] Focus keys: tab, shift+tab
- [ ] Component keys: arrows, page up/down, etc.

#### 13.3 Remove Duplicate Key Logic
- [ ] Remove duplicated switch statements
- [ ] Consolidate all key handling through new handlers

### Phase 14: Responsive Layout

#### 14.1 Implement Layout Mode Switching
- [ ] Detect compact mode in `CalculateLayout()`
- [ ] Set `layout.Mode` appropriately
- [ ] Update Footer to show mode-appropriate hints

#### 14.2 Sidebar Toggle in Compact Mode
- [ ] Add `sidebarVisible bool` field to App
- [ ] Add 's' key handler to toggle sidebar
- [ ] When visible in compact mode, overlay sidebar on main content

#### 14.3 Adapt Components to Modes
- [ ] Header: Shorten labels in compact mode
- [ ] Footer: Show condensed keybindings
- [ ] StatusBar: Truncate long task names

### Phase 15: Color Palette Refresh

#### 15.1 Define New Color Palette
- [ ] Research modern TUI color schemes (Catppuccin, Tokyo Night, etc.)
- [ ] Define primary, secondary, accent colors
- [ ] Define semantic colors (success, warning, error, info)
- [ ] Define background layers (base, subtle, overlay)

#### 15.2 Update styles.go Color Definitions
- [ ] Replace existing color constants with new palette
- [ ] Add foreground colors (base, muted, subtle)
- [ ] Add border colors (default, focused, muted)
- [ ] Document color usage guidelines in comments

#### 15.3 Apply Colors to Components
- [ ] Update header colors
- [ ] Update panel border colors
- [ ] Update status indicator colors
- [ ] Update text colors for different states

### Phase 16: Animation Support

#### 16.1 Create Animation Utilities
- [ ] Create `internal/tui/anim.go` for animation helpers
- [ ] Implement `Spinner` component with configurable frames
- [ ] Implement `Pulse` effect for highlighting changes

#### 16.2 Implement Spinner Animation
- [ ] Add spinner to StatusBar for "working" state
- [ ] Use tick-based animation (100ms intervals)
- [ ] Support multiple spinner styles (dots, braille, etc.)

#### 16.3 Implement Pulse Animation
- [ ] Add pulse effect for new inbox messages
- [ ] Add pulse effect for task status changes
- [ ] Fade pulse over 3-5 frames

#### 16.4 Animation State Management
- [ ] Track animation state in components that use them
- [ ] Start/stop animations based on app state
- [ ] Ensure animations pause when component not visible

### Phase 17: Focus and Styling Polish

#### 17.1 Focus Indicators
- [ ] Implement `borderStyle(focused bool)` in styles.go
- [ ] Use brighter border color when focused
- [ ] Add subtle background tint for focused panels

#### 17.2 Consistent Styling
- [ ] Ensure all panels use same border style
- [ ] Standardize padding (1 char horizontal, 0 vertical)
- [ ] Align text consistently across components

#### 17.3 Final Style Tweaks
- [ ] Verify color contrast meets accessibility standards
- [ ] Test colors in light and dark terminal themes
- [ ] Adjust any colors that don't render well

### Phase 18: Testing and Polish

#### 18.1 Interface Compliance Tests
- [ ] Add compile-time checks: `var _ FullComponent = (*Dashboard)(nil)`
- [ ] Add checks for all components
- [ ] Verify all interfaces are satisfied

#### 18.2 Layout Tests
- [ ] Test layout at 80x24 (minimum)
- [ ] Test layout at 120x40 (standard)
- [ ] Test layout at 200x60 (large)
- [ ] Test compact mode transitions

#### 18.3 Integration Tests
- [ ] Test keyboard navigation between components
- [ ] Test state propagation on updates
- [ ] Test viewport scrolling in all scrollable components

#### 18.4 Performance Tests
- [ ] Profile resize operations
- [ ] Ensure no lag on rapid resize
- [ ] Verify lazy layout recalculation works

#### 18.5 Fix Existing Test Issues
- [ ] Update app_test.go to remove ViewTasks references
- [ ] Update dashboard_test.go to fix undefined references
- [ ] Ensure all tests pass with new architecture

#### 18.6 Animation Tests
- [ ] Verify spinner animates correctly
- [ ] Verify pulse animations trigger on events
- [ ] Test animation performance (no frame drops)

## UI Mockup

### Desktop Mode (width >= 100)
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ iteratr │ session: my-feature │ iteration: 3/10 │ ● connected                   │
├───────────────────────────────────────────────────────┬─────────────────────────┤
│ Agent Output                                          │ Tasks                   │
│ ──────────────────────────────────────────────────────│ ─────────────────────── │
│                                                       │ ● Implement feature X   │
│ [14:32:15] Starting iteration 3...                    │ ○ Write tests           │
│ [14:32:16] Reading file src/main.go                   │ ○ Update docs           │
│ [14:32:18] Editing function handleRequest             │                         │
│ [14:32:20] Running tests...                           │ Notes                   │
│ [14:32:25] All tests passed                           │ ─────────────────────── │
│                                                       │ tip: Use context for... │
│                                                       │ decision: Chose approach│
│                                                  75%  │                    50%  │
├───────────────────────────────────────────────────────┴─────────────────────────┤
│ ◐ Working │ task: Implement feature X                                           │
├─────────────────────────────────────────────────────────────────────────────────┤
│ [1]Dashboard [2]Logs [3]Notes [4]Inbox │ [tab]Focus [?]Help [q]Quit             │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Compact Mode (width < 100)
```
┌────────────────────────────────────────────────────┐
│ iteratr │ my-feature │ 3/10 │ ●                    │
├────────────────────────────────────────────────────┤
│ Agent Output                                       │
│ ───────────────────────────────────────────────────│
│                                                    │
│ [14:32:15] Starting iteration 3...                 │
│ [14:32:16] Reading file src/main.go                │
│ [14:32:18] Editing function handleRequest          │
│ [14:32:20] Running tests...                        │
│ [14:32:25] All tests passed                        │
│                                               75%  │
├────────────────────────────────────────────────────┤
│ ◐ Working │ Implement feature X                    │
├────────────────────────────────────────────────────┤
│ [1-4]Views [s]Sidebar [?]Help [q]Quit              │
└────────────────────────────────────────────────────┘
```

## Out of Scope

- Theme switching (light/dark mode) - foundation only, no runtime switching
- Mouse click handling for navigation
- Dialog/modal overlay system - foundation only
- Custom color configuration by users
- Persistent layout preferences
- Complex animation sequences (only spinner + pulse)

## Decisions Made

1. **Bubbletea v2 View API**: `View()` returns `tea.View` struct with `Content`, `Cursor`, etc. Use `uv.NewScreenBuffer()` + `canvas.Render()` pattern.

2. **Sidebar Content**: Contains both Tasks (60%) and Notes (40%) sections.

3. **Color Scheme**: Refresh with modern, cohesive palette (not keeping old colors).

4. **Animation Support**: Spinner for working state + pulse for new messages/updates.

5. **Test Failures**: Fix existing test issues (ViewTasks, taskStats) in Phase 18.

6. **Dashboard Composition**: Agent output in main area, sidebar shows tasks + notes preview.

## Open Questions

1. **What's the minimum supported terminal size?**
   - Propose: 80x24 (standard terminal minimum)

2. **Should sidebar be toggleable in desktop mode too?**
   - Defer: Start with responsive-only, add toggle if requested

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Ultraviolet Screen/Draw learning curve | Medium | Query btca, follow Crush patterns closely |
| Breaking existing functionality | High | Incremental refactor, test each phase |
| Performance regression | Medium | Profile after each phase, lazy layout calc |
| Interface proliferation | Low | Keep interfaces minimal, compose as needed |
| Animation performance | Low | Use tick-based updates, pause when not visible |
| Color accessibility | Medium | Test with colorblind simulators, ensure contrast |

## References

- Crush TUI patterns: `btca ask -r crush -q "Screen Draw pattern"`
- Ultraviolet layouts: `btca ask -r ultraviolet -q "SplitVertical SplitHorizontal"`
- Lipgloss styling: `btca ask -r lipgloss -q "styling borders"`
- Bubbles viewport: `btca ask -r bubbles -q "viewport component"`
- Current codebase: `internal/tui/`
