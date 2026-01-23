# User Input via ACP (Replace Inbox)

## Overview

Remove the inbox concept (TUI panel, CLI command, session state, tool commands, template placeholder) and replace it with a Bubbles `textinput` component embedded directly in the agent output view. User messages are sent as `session/prompt` calls to a persistent ACP session.

## User Story

As a user watching the agent work, I want to type a message in an input field below the agent output and have it delivered directly to the running ACP session, without needing a separate inbox tab or CLI command.

## Requirements

- Bubbles v2 `textinput.Model` below the agent output viewport (matches crush pattern)
- Input focused with `i`, unfocused with `Escape`
- `Enter` sends message as `session/prompt` to the active ACP session
- ACP subprocess kept alive between prompts (persistent session)
- Remove: inbox TUI panel, `[4] Inbox` footer tab, `ViewInbox` enum
- Remove: `iteratr message` CLI command
- Remove: `iteratr tool inbox-list` and `iteratr tool inbox-mark-read` subcommands
- Remove: `session.State.Inbox`, `session.InboxAdd/MarkRead/List`, `EventTypeInbox`
- Remove: `{{inbox}}` template variable, inbox instructions in default template

## Technical Implementation

### Architecture Change

**Before (Inbox):**
```
User → `iteratr message` CLI → NATS inbox event → next iteration prompt includes inbox
Agent → `iteratr tool inbox-list` → reads queued messages
```

**After (Direct ACP Input):**
```
User → TUI input field → session/prompt → agent receives message in same session
```

### Key Design Decision: Persistent ACP Session

Currently each iteration spawns a new `opencode acp` process. To support user input mid-session, the runner must:
1. Keep the ACP subprocess alive across prompts
2. Store the `acpConn` and `sessionID` for reuse
3. Allow sending additional `session/prompt` calls from the TUI input

### ACP Methods Used

- `session/prompt` - sends user message (same method as initial prompt). Per ACP spec: "Once a prompt turn completes, the Client may send another session/prompt to continue the conversation, building on the context established in previous turns."
- `session/cancel` - notification to abort a running turn. Agent responds with `stopReason: "cancelled"` to the original prompt request. (Future: interrupt agent to send urgent user message)

### New Dependencies

- `charm.land/bubbles/v2/textinput` (already available in go.mod via bubbles)

### Key Files

**Create:**
- `internal/tui/scrolllist.go` - Lazy-rendering ScrollList component (shared by all panes)

**Modify:**
- `internal/tui/agent.go` - Add textinput, replace viewport with ScrollList, split layout
- `internal/tui/dashboard.go` - FocusPane enum, Tab-cycle, global `i`/Enter/Escape, mouse click on input
- `internal/tui/sidebar.go` - Replace viewports with ScrollList, task cursor via j/k
- `internal/tui/app.go` - Remove inbox, add UserInputMsg handling, wire send
- `internal/tui/draw.go` - Update DrawPanel() to accept focus state for accent border
- `internal/tui/footer.go` - Remove inbox button, renumber
- `internal/tui/interfaces.go` - Remove ViewInbox
- `internal/tui/styles.go` - Remove inbox styles, add input/focus styles
- `internal/tui/logs.go` - Remove inbox log type styling
- `internal/agent/runner.go` - Persistent ACP session, expose SendMessage
- `internal/agent/acp.go` - Add sendUserMessage method
- `internal/orchestrator/orchestrator.go` - Expose SendMessage to TUI
- `internal/session/session.go` - Remove Inbox field, applyInboxEvent
- `internal/nats/store.go` - Remove EventTypeInbox
- `internal/template/template.go` - Remove Inbox var, formatInbox
- `internal/template/default.go` - Remove inbox instructions
- `cmd/iteratr/main.go` - Remove messageCmd
- `cmd/iteratr/tool.go` - Remove inboxListCmd, inboxMarkReadCmd

**Delete:**
- `internal/tui/inbox.go`
- `internal/tui/inbox_pulse_test.go`
- `internal/session/inbox.go`
- `internal/session/inbox_test.go`
- `cmd/iteratr/message.go`

## Tasks

### 1. Tracer bullet: Persistent ACP session in runner

Keep the ACP subprocess alive and expose a method to send follow-up messages.

- [ ] Add `conn *acpConn`, `sessionID string`, `cmd *exec.Cmd` fields to `Runner` struct
- [ ] Extract ACP setup (spawn, initialize, newSession, setModel) into `Runner.Start(ctx) error`
- [ ] Extract ACP teardown into `Runner.Stop()`
- [ ] Change `RunIteration(ctx, prompt)` to only call `conn.prompt(...)` (reuse existing connection)
- [ ] Add `Runner.SendMessage(ctx context.Context, text string) error` that calls `conn.prompt(ctx, sessionID, text, onText, onToolCall)`
- [ ] Update orchestrator to call `runner.Start()` before iteration loop and `runner.Stop()` on exit

### 2. Tracer bullet: Add textinput to AgentOutput

Add the Bubbles textinput below the viewport, wired to return a command with the user's message.

- [ ] Add `input textinput.Model` field to `AgentOutput` (no `inputActive` — focus owned by Dashboard)
- [ ] Initialize textinput in `NewAgentOutput()`: placeholder "Send a message...", prompt "> ", virtual cursor off, dark styles
- [ ] In `UpdateSize()`: split height into viewport (h-3) and input area (3 lines); set `input.SetWidth(width-4)`
- [ ] In `Draw()`: use `uv.SplitVertical(area, uv.Flex(1), uv.Fixed(3))` for viewport vs input
- [ ] Draw separator line (─) and `input.View()` in the input area
- [ ] Return `input.Cursor()` from `Draw()` when input is focused
- [ ] Add `SetInputFocused(focused bool)`: calls `input.Focus()` or `input.Blur()`
- [ ] In `Update()`: if input focused, forward msgs to `input.Update(msg)` (Dashboard handles Enter/Escape/Tab before forwarding)
- [ ] Add `InputValue() string` and `ResetInput()` helpers for Dashboard to read/clear input

### 3. Tracer bullet: Wire input to ACP

Connect the TUI input message to the runner's SendMessage. Includes minimal Dashboard key handling for tracer bullet (task 14 refines into full Tab-cycle).

- [ ] Define `UserInputMsg struct { Text string }` in `app.go`
- [ ] Minimal Dashboard key handling (tracer bullet): `i` → `agentOutput.SetInputFocused(true)`, Enter → emit `UserInputMsg{agentOutput.InputValue()}` + `ResetInput()`, Escape → `SetInputFocused(false)`. Task 14 replaces this with full FocusPane system
- [ ] Handle `UserInputMsg` in `app.go` Update: send message to orchestrator
- [ ] Add `sendChan chan string` to `App` struct, pass to orchestrator
- [ ] In orchestrator iteration loop: after each iteration completes, `select` on `sendChan` before starting next auto-iteration. User message takes priority over next auto-iteration prompt
- [ ] When user message received: call `runner.SendMessage(ctx, text)`, wait for response, then resume iteration loop
- [ ] Verify end-to-end: type in input → agent receives and responds → response appears in viewport

### 4. Remove inbox TUI panel

- [ ] Delete `internal/tui/inbox.go`
- [ ] Delete `internal/tui/inbox_pulse_test.go`
- [ ] Remove `inbox *InboxPanel` field from `App` struct in `app.go`
- [ ] Remove `inbox: NewInboxPanel()` from `NewApp()`
- [ ] Remove `a.inbox.UpdateState(msg.State)` from state update handling
- [ ] Remove inbox pulse animation logic from `Update()` (lines 177-206 inboxCmd handling)
- [ ] Remove `a.inbox.SetSize(...)` from `propagateSizes()`

### 5. Remove ViewInbox and footer button

- [ ] Remove `ViewInbox` from `ViewType` enum in `interfaces.go`
- [ ] Remove `FooterActionInbox` from footer.go constants
- [ ] Remove `{"4", "Inbox", ViewInbox, FooterActionInbox}` from footer view buttons
- [ ] Remove `case FooterActionInbox:` from `handleFooterAction()` in app.go
- [ ] Remove `case "4":` → ViewInbox from `handleViewKeys()` in app.go
- [ ] Remove `case ViewInbox:` from `delegateToActive()` in app.go
- [ ] Remove `case ViewInbox:` from `drawActiveView()` in app.go
- [ ] Remove `styleLogInbox` usage from `logs.go` (inbox log type case)
- [ ] Remove inbox-related styles from `styles.go`

### 6. Remove message CLI command

- [ ] Delete `cmd/iteratr/message.go`
- [ ] Remove `rootCmd.AddCommand(messageCmd)` from `cmd/iteratr/main.go`
- [ ] Remove `connectToNATS()` helper from message.go (check if used elsewhere first)

### 7. Remove inbox tool subcommands

- [ ] Remove `toolCmd.AddCommand(inboxListCmd)` from `cmd/iteratr/tool.go`
- [ ] Remove `toolCmd.AddCommand(inboxMarkReadCmd)` from `cmd/iteratr/tool.go`
- [ ] Remove `inboxListCmd` variable and command definition
- [ ] Remove `inboxMarkReadCmd` variable, command definition, and init() flags

### 8. Remove inbox from session state

- [ ] Delete `internal/session/inbox.go`
- [ ] Delete `internal/session/inbox_test.go`
- [ ] Remove `Inbox []*Message` field from `State` struct in `session.go`
- [ ] Remove `case nats.EventTypeInbox:` and `st.applyInboxEvent(event)` from `State.Apply()`
- [ ] Remove `applyInboxEvent()` method from `session.go`
- [ ] Remove `Message` struct from session.go (if not used elsewhere)
- [ ] Remove `EventTypeInbox` constant from `internal/nats/store.go`

### 9. Remove inbox from template system

- [ ] Remove `Inbox string` field from `TemplateVars` struct in `template.go`
- [ ] Remove `"{{inbox}}": vars.Inbox` from placeholder map
- [ ] Remove `formatInbox()` function
- [ ] Remove `len(state.Inbox)` from debug log
- [ ] Remove `Inbox: formatInbox(state)` from `BuildVarsFromState()`
- [ ] Remove `{{inbox}}` from default template in `default.go`
- [ ] Remove inbox instructions section ("Review inbox above", inbox-mark-read tool reference) from default template
- [ ] Remove `InboxAddParams` and `InboxMarkReadParams` type references from session package

### 10. Input styling (match crush)

- [ ] Add `styleInputSeparator` - dim horizontal line using `─` chars
- [ ] Add textinput styles: focused text (base), placeholder (fgSubtle), prompt (tertiary/accent)
- [ ] Add cursor style: bar shape, accent color, blink enabled
- [ ] Add help text below input when focused: "Enter to send, Esc to cancel"
- [ ] Add help text when unfocused: "Press i to type a message"

### 11. Fix tests

- [ ] Update `app_test.go`: remove inbox nil check, remove ViewInbox test case
- [ ] Update `integration_test.go`: remove inbox view switch test, remove inbox state test, remove inbox focus test
- [ ] Update `app_animation_test.go`: remove inbox pulse tests
- [ ] Update `template_test.go`: remove Inbox field from test vars, remove `TestFormatInbox`
- [ ] Add test: `AgentOutput` input field renders correctly
- [ ] Add test: `UserInputMsg` is produced on Enter
- [ ] Add test: input focus/blur with i/Escape
- [ ] Verify build compiles: `go build ./...`
- [ ] Verify tests pass: `go test ./...`

### 12. Guard input when agent is busy

ACP requires each `session/prompt` to complete before the next can be sent. Queue user input during an active turn.

- [ ] Add `agentBusy bool` field to `Dashboard` (not AgentOutput — Dashboard owns input flow)
- [ ] Set `agentBusy = true` when iteration starts (IterationStartMsg)
- [ ] Set `agentBusy = false` when agent finishes (prompt returns, AgentFinishMsg)
- [ ] When busy: AgentOutput shows "Agent is working..." as input placeholder
- [ ] On Enter while busy: Dashboard stores message in `queuedMsg string`, AgentOutput shows "(queued)" indicator
- [ ] When not busy and `queuedMsg != ""`: Dashboard emits `UserInputMsg` immediately, clears queue

### 13. Replace viewport with lazy-rendering ScrollList

The `bubbles/v2/viewport` re-processes the entire content string on every `SetContent()` call. During streaming, `refreshContent()` is called per text chunk — iterating all messages, re-rendering, and concatenating into a full string. Replace with a custom offset-based list that only renders visible items. This component is shared across all three scrollable panes: agent output, tasks, and notes.

- [ ] Create `internal/tui/scrolllist.go` with `ScrollList` struct: fields `items []ScrollItem`, `offsetIdx int`, `offsetLine int`, `width int`, `height int`, `autoScroll bool`, `focused bool`
- [ ] Define `ScrollItem` interface: `ID() string`, `Render(width int) string`, `Height() int` (same as `MessageItem` — alias or embed)
- [ ] Implement `ScrollList.View() string`: iterate from `offsetIdx` forward, render items until viewport height filled, skip `offsetLine` lines from first visible item
- [ ] Implement `ScrollList.ScrollBy(lines int)`: adjust `offsetIdx`/`offsetLine`, clamp to bounds, respect item heights
- [ ] Implement `ScrollList.GotoBottom()`: set offset so last item's last line is at bottom of viewport
- [ ] Implement `ScrollList.AtBottom() bool`: check if offset is at max scroll position
- [ ] Implement `ScrollList.TotalLineCount() int`: sum cached heights of all items
- [ ] Implement `ScrollList.ScrollPercent() float64`: current offset / max offset
- [ ] Implement `ScrollList.Update(msg tea.Msg) tea.Cmd`: handle `tea.KeyPressMsg` (pgup/pgdn/home/end) only when `focused == true`. Note: `j`/`k` NOT bound here — tasks pane uses them for cursor, agent pane doesn't need them
- [ ] Add `ScrollList.selectedIdx int` field and `SetSelected(idx int)`. When rendering, ScrollList applies selection styling (e.g., `▸` prefix + highlight background) externally around the selected item's output — items don't need to know about selection
- [ ] Replace `viewport.Model` in `AgentOutput` with `ScrollList`
- [ ] Update `refreshContent()` → only call `GotoBottom()` if `autoScroll` (no full re-render needed — list renders lazily)
- [ ] Optimize streaming: `AppendText()` should only invalidate the last item, not trigger full list render
- [ ] Replace `tasksViewport viewport.Model` in `Sidebar` with `ScrollList` for tasks
- [ ] Replace `notesViewport viewport.Model` in `Sidebar` with `ScrollList` for notes
- [ ] Wrap task/note rendered lines as `ScrollItem` (simple `TextScrollItem` wrapping a pre-rendered string)

### 14. Tab-cycle focus between panes

Replace the current binary main/sidebar focus with a multi-pane Tab-cycle (like lazygit/lazydocker). Focused pane intercepts keyboard scroll events. Dashboard owns all focus state.

- [ ] Define `FocusPane` enum: `FocusAgent`, `FocusTasks`, `FocusNotes`, `FocusInput` (replace `FocusMain`/`FocusSidebar`)
- [ ] `i` key is global: handled at Dashboard level, sets `FocusInput` from any pane, calls `agentOutput.SetInputFocused(true)`
- [ ] Tab: intercept before forwarding to any child. If `FocusInput` → exit input, set `FocusAgent`. Otherwise cycle `FocusAgent → FocusTasks → FocusNotes → FocusAgent`
- [ ] Escape: if `FocusInput` → exit input, set `FocusAgent`. Otherwise no-op
- [ ] Enter: if `FocusInput` → read `agentOutput.InputValue()`, emit `UserInputMsg`, call `agentOutput.ResetInput()`, set `FocusAgent`
- [ ] Set `ScrollList.focused = true` only on the active pane's list; clear others
- [ ] Focused pane: border highlight (accent color)
- [ ] `FocusAgent`: pgup/pgdn/home/end forwarded to agent ScrollList
- [ ] `FocusTasks`: `j`/`k` moves task cursor (Sidebar handles), pgup/pgdn scrolls tasks ScrollList
- [ ] `FocusNotes`: pgup/pgdn scrolls notes ScrollList
- [ ] Unfocused panes: dim border, no keyboard interception
- [ ] Mouse click on text input area → set `FocusInput`, call `agentOutput.SetInputFocused(true)`
- [ ] Update `DrawPanel()` to accept focus state and render accent border when focused
- [ ] Remove old `FocusArea` type and `FocusMain`/`FocusSidebar` constants

## UI Mockup

```
┌─ Agent Output ────────────────────────────────────┐
│                                                    │
│  Agent's streaming text output here...             │
│  ✓ bash                                            │
│    command: go build ./...                         │
│                                                    │
│  More agent text...                                │
│                                                    │
├────────────────────────────────────────────────────┤
│ > Send a message...                          100%  │
└────────────────────────────────────────────────────┘
```

When focused:
```
├────────────────────────────────────────────────────┤
│ > Fix the type error in main.go█                   │
│ Enter to send · Esc to cancel                      │
└────────────────────────────────────────────────────┘
```

## Out of Scope

- Multi-line text input (textarea) — single line is sufficient for v1
- Message history (up arrow to recall previous messages)
- Interrupting a running prompt with user message (session/cancel + re-prompt)
- File/image attachments in user messages
- Showing user messages in the agent output viewport
- Auto-complete or suggestions in the input field

## Open Questions

None.

## Resolved

- Can `session/prompt` be sent while a previous prompt is streaming? **No.** Per ACP spec, the prompt response (with `stopReason`) arrives only after the turn fully completes. Must queue user messages and send after current turn finishes. The `session/cancel` notification can abort a turn early if needed.
