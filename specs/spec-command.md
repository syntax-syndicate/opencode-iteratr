# Spec Command

## Overview

New `iteratr spec` subcommand with wizard UI for creating feature specs via AI-assisted interview. Spawns opencode acp with custom MCP server (`iteratr-spec`) exposing question/finish tools.

## User Story

Developer wants to create a well-structured spec without manually writing markdown. Wizard collects name/description, then AI agent interviews user in depth about requirements, edge cases, and tradeoffs before generating complete spec.

## Requirements

### Wizard Flow
1. **Title Input** - Single-line text, slug format validation (lowercase alphanumeric + hyphens)
2. **Description Textarea** - Multi-line, no limit, hint: "provide as much detail as possible"
3. **Model Selector** - Reuse from build wizard (`internal/tui/wizard/model_selector.go`)
4. **Agent Phase** - Auto-start after model selection, interact via question tool until finish-spec called
5. **Wait for Agent Stop** - Agent session ends, MCP server shut down
6. **Save Spec** - Write content to `{spec_dir}/{slug}.md`, update README.md
7. **Next Steps** - View/Start Build/Exit buttons

### Agent Phase UI
- Spinner with status text while agent thinking (e.g., "Agent is analyzing...")
- Agent text output hidden from user
- Questions displayed one at a time (not batch)
- Show "Question X of Y" counter
- Track selected answers per question with persistence
- Answer persistence: user can navigate back/forward between questions, answers are preserved
  - Example: Answer Q1 → Answer Q2 → Back to Q1 → Change answer → Forward to Q2 → Q2 answer still there
- Navigation: Back/Next buttons to navigate between questions (in addition to Submit on last question)
- No timeout on user responses
- Full tab navigation between all focusable widgets (buttons, radio options, text inputs, Back/Next)
- ESC triggers "Are you sure you want to cancel?" confirmation

### MCP Server: iteratr-spec
**MUST be separate process from build's `iteratr-tools` MCP server**. Runs on random available port. Only exposes tools needed for spec creation.

**ask-questions** - Matches OpenCode question tool API exactly
```
Parameters:
  questions: array of {
    question: string     // Full question text
    header: string       // Short label (max 30 chars)
    options: array of {
      label: string      // Display text (1-5 words)
      description: string
    }
    multiple?: bool      // Allow multi-select (default: false)
  }

Behavior:
- Show questions one at a time
- Auto-append "Type your own answer" option to all questions (custom=true by default in OpenCode)
  - For multi-select: "Type your own" is mutually exclusive (deselects all other options when selected)
- Track question count (current/total) for UI display
- When user selects "Type your own answer", show text input
- Reject empty custom responses (show error, re-prompt)
- Block MCP handler until all questions answered
- Return array of answers where each answer is:
  - Single-select: string (option label or custom text)
  - Multi-select: string array (array of option labels, or array with single custom text)
```

**finish-spec**
```
Parameters:
  content: string   // Full spec markdown content (required, only parameter)

Behavior:
- Use wizard title (passed from wizard, NOT extracted from content)
- Validate content: check for "## Overview" and "## Tasks" headings (case-insensitive, level 2 headings)
  - If either missing: return error listing which sections are missing
- Slugify title (spaces->hyphens, lowercase, transliterate accents)
- Check if file exists at {spec_dir}/{slug}.md
  - If exists: return error with message asking agent to confirm overwrite or choose new name
- Save to {spec_dir}/{slug}.md
- Update README.md with spec entry (see README section)
- Return success message with file path

Note: The wizard title is set in step 1 and passed to MCP server during initialization. The
finish-spec handler receives it via server state, NOT by parsing content.
```

### README.md Update
- Look for `<!-- SPECS -->` marker
- If found: insert row after marker
- If not found: append marker + new table after existing content
- Table format: `| [Name](slug.md) | Description | Date |`
  - Name: Link to spec file using slug
  - Description: First line from wizard description field (up to 100 chars)
  - Date: Current date in YYYY-MM-DD format
- Create README with header + table if missing

### Completion Screen
Buttons after spec saved:
- **View**: Open in $EDITOR using `github.com/charmbracelet/x/editor` with `tea.ExecProcess` (only show if $EDITOR set)
- **Start Build**: Execute `iteratr build --spec <path>` directly
- **Exit**: Return to shell

Note: If $EDITOR is not set, View button is hidden and spec path is displayed in the success message.

### Configuration
- `spec_dir` in iteratr.yml (default: `./specs`)
- `ITERATR_SPEC_DIR` env var

### Error Handling
- opencode acp start failure: show error message, exit wizard
- Agent ends without calling finish-spec: discard everything, show error
- File exists on save: MCP returns error, agent should ask user to confirm overwrite or provide new name
- ESC during questions: go back to previous question if not on first, otherwise show cancel confirmation
- ESC during spinner: show cancel confirmation modal

### Agent Prompt
```
Follow the user instructions and interview me in detail using the ask-questions 
tool about literally anything: technical implementation, UI & UX, concerns, 
tradeoffs, etc. but make sure the questions are not obvious. Be very in-depth 
and continue interviewing me continually until it's complete. Then, write the 
spec using the finish-spec tool.

Feature: {name}
Description: {description}

## Spec Format
[Include full spec format from AGENTS.md]
```

## Technical Implementation

### New Files
- `cmd/iteratr/spec.go` - Cobra command setup
- `internal/tui/specwizard/wizard.go` - Main wizard model with step management
- `internal/tui/specwizard/title_step.go` - Title input step (slug validation)
- `internal/tui/specwizard/description_step.go` - Textarea step (reuse textarea patterns from template_editor.go)
- `internal/tui/specwizard/agent_phase.go` - Agent interaction view with spinner + question handling
- `internal/tui/specwizard/completion_step.go` - Final actions view with View/Build/Exit buttons
- `internal/tui/specwizard/question_view.go` - Single question component with tab navigation, radio buttons, custom input
- `internal/specmcp/server.go` - MCP HTTP server (pattern from internal/mcpserver/server.go)
- `internal/specmcp/tools.go` - Tool registration for ask-questions and finish-spec
- `internal/specmcp/handlers.go` - Tool handler implementations with channel-based question/answer flow

### Reused Components
**Directly reuse these existing components from build wizard:**
- `internal/tui/wizard/model_selector.go` - Model selection step (call NewModelSelectorStep())
- `internal/tui/wizard/button_bar.go` - Navigation buttons with focus management (NewButtonBar, Focus/Blur methods)
- `internal/tui/wizard/styles.go` - Theme and styling helpers
- `internal/agent/runner.go` - ACP spawning (create stateless runner, no session store)
- `internal/agent/acp.go` - ACP protocol implementation

### Data Flow
```
User Input (Title) -> User Input (Description) -> Model Selected
                                                  |
                                    Start MCP server (iteratr-spec on random port)
                                    Spawn opencode acp process (stdin/stdout JSON-RPC)
                                                  |
                                    Send initialize request (ACP protocol handshake)
                                                  |
                                    Send session/new with mcpServers: [{"type": "http", "url": "http://localhost:{port}/mcp"}]
                                                  |
                                    Send session/prompt with spec creation instructions
                                                  |
Agent calls ask-questions ----------> MCP handler blocks -> Send question to UI via channel
                                                  |
                                    UI renders question view with options
                                                  |
User selects/types answer ----------> Send answer back via channel -> MCP returns result to agent
                                                  |
                                    Agent continues thinking/asking more questions
                                                  |
Agent calls finish-spec ------------> Validate content -> Save {spec_dir}/{slug}.md
                                                  |
                                    Update README.md with new entry
                                                  |
                                    session/prompt returns with stopReason -> Clean shutdown
                                                  |
                                    Completion screen: View / Build / Exit
```

### Code Examples

#### 1. MCP Server Pattern (internal/specmcp/server.go)
```go
package specmcp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"github.com/mark3labs/mcp-go/server"
)

// Server manages the spec-specific MCP server
type Server struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	port       int
	mu         sync.Mutex
	
	// Wizard state (passed during initialization)
	specTitle string // Title from wizard step 1
	specDir   string // From config
	
	// Channels for question/answer flow
	questionCh chan QuestionRequest
	answerCh   chan AnswerResponse
}

type QuestionRequest struct {
	Questions []Question
	ResultCh  chan []interface{} // Return answers here (string or []string per question)
}

type AnswerResponse struct {
	Answers []interface{} // Each element is string (single-select) or []string (multi-select)
	Err     error
}

func New(specTitle, specDir string) *Server {
	return &Server{
		specTitle:  specTitle,
		specDir:    specDir,
		questionCh: make(chan QuestionRequest, 1),
		answerCh:   make(chan AnswerResponse, 1),
	}
}

func (s *Server) Start(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Create MCP server
	s.mcpServer = server.NewMCPServer(
		"iteratr-spec",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	
	// Register tools
	if err := s.registerTools(); err != nil {
		return 0, err
	}
	
	// Find random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	s.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	
	// Start HTTP server
	s.httpServer = server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithStateLess(true),
	)
	
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	go s.httpServer.Start(addr)
	
	return s.port, nil
}

// QuestionChan returns channel for receiving question requests from MCP handlers
func (s *Server) QuestionChan() <-chan QuestionRequest {
	return s.questionCh
}

// SendAnswers sends answers back to waiting MCP handler
func (s *Server) SendAnswers(answers []interface{}, err error) {
	s.answerCh <- AnswerResponse{Answers: answers, Err: err}
}
```

#### 2. Question Tool Handler (internal/specmcp/handlers.go)
```go
func (s *Server) handleAskQuestions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	questionsRaw, ok := args["questions"].([]any)
	if !ok {
		return mcp.NewToolResultText("error: invalid questions parameter"), nil
	}
	
	// Parse questions
	questions := make([]Question, 0, len(questionsRaw))
	for _, qRaw := range questionsRaw {
		qMap := qRaw.(map[string]any)
		// Parse question, header, options, multiple flag
		// ...
		questions = append(questions, question)
	}
	
	// Send questions to UI and block for answers
	resultCh := make(chan []interface{}, 1)
	s.questionCh <- QuestionRequest{
		Questions: questions,
		ResultCh:  resultCh,
	}
	
	// Block until answers received from UI
	select {
	case answers := <-resultCh:
		// Return answers as JSON array (each element is string or []string)
		answersJSON, _ := json.Marshal(answers)
		return mcp.NewToolResultText(string(answersJSON)), nil
	case <-ctx.Done():
		return mcp.NewToolResultText("error: cancelled"), nil
	}
}
```

#### 3. REMOVED - See Section 6 for the complete implementation with QuestionAnswer type

#### 6. Complete Question View Integration Example
**Full implementation showing how to integrate option selection with navigation and persistence:**

```go
// QuestionAnswer stores a single answer (handles both single and multi-select)
type QuestionAnswer struct {
	Value   interface{} // string for single-select, []string for multi-select
	IsMulti bool
}

type QuestionView struct {
	questions      []Question
	answers        []QuestionAnswer // Persistent answers (parallel to questions)
	currentIndex   int
	
	// Option selection component (from pattern above)
	optionSelector QuestionOptions
	
	// Custom text input
	customInput    textinput.Model
	showCustom     bool
	
	// Focus management
	focusIndex     int // 0=options, 1=custom, 2=back, 3=next
	
	width  int
	height int
}

func NewQuestionView(questions []Question, answers []QuestionAnswer, currentIndex int) *QuestionView {
	q := questions[currentIndex]
	
	// Build option items from question
	optItems := make([]OptionItem, len(q.Options))
	for i, opt := range q.Options {
		optItems[i] = OptionItem{
			label:       opt.Label,
			description: opt.Description,
			selected:    false,
		}
	}
	// Add "Type your own answer" option
	optItems = append(optItems, OptionItem{
		label:       "Type your own answer",
		description: "Enter custom text",
		selected:    false,
	})
	
	qv := &QuestionView{
		questions:      questions,
		answers:        answers,
		currentIndex:   currentIndex,
		optionSelector: NewQuestionOptions(optItems, q.Multiple),
		customInput:    textinput.New(),
		focusIndex:     0,
	}
	
	// Restore previous answer
	qv.restoreAnswer()
	qv.optionSelector.Focus()
	
	return qv
}

func (q *QuestionView) restoreAnswer() {
	answer := q.answers[q.currentIndex]
	currentQ := q.questions[q.currentIndex]
	
	if answer.IsMulti {
		// Multi-select answer
		if labels, ok := answer.Value.([]string); ok {
			if len(labels) == 0 {
				return
			}
			
			// Check if it's a custom answer
			if len(labels) == 1 && !q.isOptionLabel(labels[0]) {
				// Custom text
				q.customInput.SetValue(labels[0])
				q.showCustom = true
				lastIdx := len(q.optionSelector.items) - 1
				q.optionSelector.items[lastIdx].selected = true
			} else {
				// Regular options
				for _, label := range labels {
					for i, opt := range currentQ.Options {
						if opt.Label == label {
							q.optionSelector.items[i].selected = true
						}
					}
				}
			}
		}
	} else {
		// Single-select answer
		if label, ok := answer.Value.(string); ok && label != "" {
			// Try to match to option
			found := false
			for i, opt := range currentQ.Options {
				if opt.Label == label {
					q.optionSelector.items[i].selected = true
					found = true
					break
				}
			}
			
			if !found {
				// Must be custom text
				q.customInput.SetValue(label)
				q.showCustom = true
				lastIdx := len(q.optionSelector.items) - 1
				q.optionSelector.items[lastIdx].selected = true
			}
		}
	}
}

func (q *QuestionView) isOptionLabel(label string) bool {
	for _, opt := range q.questions[q.currentIndex].Options {
		if opt.Label == label {
			return true
		}
	}
	return false
}

func (q *QuestionView) saveCurrentAnswer() {
	selected := q.optionSelector.SelectedLabels()
	currentQ := q.questions[q.currentIndex]
	
	if len(selected) == 0 {
		// No selection
		q.answers[q.currentIndex] = QuestionAnswer{Value: "", IsMulti: currentQ.Multiple}
		return
	}
	
	// Check if "Type your own" is selected
	isCustom := selected[0] == "Type your own answer"
	
	if currentQ.Multiple {
		// Multi-select answer
		if isCustom {
			customText := q.customInput.Value()
			q.answers[q.currentIndex] = QuestionAnswer{
				Value:   []string{customText},
				IsMulti: true,
			}
		} else {
			// Remove "Type your own answer" if somehow in list
			filtered := make([]string, 0, len(selected))
			for _, s := range selected {
				if s != "Type your own answer" {
					filtered = append(filtered, s)
				}
			}
			q.answers[q.currentIndex] = QuestionAnswer{
				Value:   filtered,
				IsMulti: true,
			}
		}
	} else {
		// Single-select answer
		if isCustom {
			q.answers[q.currentIndex] = QuestionAnswer{
				Value:   q.customInput.Value(),
				IsMulti: false,
			}
		} else {
			q.answers[q.currentIndex] = QuestionAnswer{
				Value:   selected[0],
				IsMulti: false,
			}
		}
	}
}

func (q *QuestionView) maxFocusIndex() int {
	// Calculate max focus index dynamically
	// 0 = options (always present)
	max := 0
	if q.showCustom {
		max++ // 1 = custom input
	}
	if q.currentIndex > 0 {
		max++ // Back button (only after first question)
	}
	max++ // Next/Submit button (always present)
	return max
}

func (q *QuestionView) backButtonFocusIndex() int {
	idx := 1
	if q.showCustom {
		idx++
	}
	return idx
}

func (q *QuestionView) nextButtonFocusIndex() int {
	idx := q.backButtonFocusIndex()
	if q.currentIndex > 0 {
		idx++
	}
	return idx
}

func (q *QuestionView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			q.focusIndex++
			// Skip custom input if not visible
			if q.focusIndex == 1 && !q.showCustom {
				q.focusIndex++
			}
			// Skip back button if on first question
			if q.focusIndex == q.backButtonFocusIndex() && q.currentIndex == 0 {
				q.focusIndex++
			}
			// Wrap around
			if q.focusIndex > q.maxFocusIndex() {
				q.focusIndex = 0
			}
			q.updateFocus()
			return nil
			
		case "shift+tab":
			q.focusIndex--
			// Skip back button if on first question
			if q.focusIndex == q.backButtonFocusIndex() && q.currentIndex == 0 {
				q.focusIndex--
			}
			// Skip custom input if not visible
			if q.focusIndex == 1 && !q.showCustom {
				q.focusIndex--
			}
			// Wrap around
			if q.focusIndex < 0 {
				q.focusIndex = q.maxFocusIndex()
			}
			q.updateFocus()
			return nil
		}
	}
	
	// Route to focused component
	if q.focusIndex == 0 {
		// Options focused
		var cmd tea.Cmd
		q.optionSelector, cmd = q.optionSelector.Update(msg)
		
		// Check if "Type your own" was selected
		selected := q.optionSelector.SelectedLabels()
		if len(selected) > 0 && selected[0] == "Type your own answer" {
			q.showCustom = true
		}
		
		return cmd
	} else if q.focusIndex == 1 && q.showCustom {
		// Custom input focused
		var cmd tea.Cmd
		q.customInput, cmd = q.customInput.Update(msg)
		return cmd
	} else if q.focusIndex == q.backButtonFocusIndex() && q.currentIndex > 0 {
		// Back button focused
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "enter" || keyMsg.String() == " " {
				q.saveCurrentAnswer()
				return func() tea.Msg { return PrevQuestionMsg{} }
			}
		}
	} else if q.focusIndex == q.nextButtonFocusIndex() {
		// Next/Submit button focused
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "enter" || keyMsg.String() == " " {
				if !q.validateAnswer() {
					return func() tea.Msg {
						return ShowErrorMsg{err: "Please select an answer"}
					}
				}
				q.saveCurrentAnswer()
				
				if q.currentIndex == len(q.questions)-1 {
					return func() tea.Msg { return SubmitAnswersMsg{} }
				}
				return func() tea.Msg { return NextQuestionMsg{} }
			}
		}
	}
	
	return nil
}

func (q *QuestionView) updateFocus() {
	// Blur all components
	q.optionSelector.Blur()
	q.customInput.Blur()
	
	// Focus active component
	switch q.focusIndex {
	case 0:
		q.optionSelector.Focus()
	case 1:
		q.customInput.Focus()
	}
}

func (q *QuestionView) validateAnswer() bool {
	selected := q.optionSelector.SelectedLabels()
	if len(selected) == 0 {
		return false
	}
	
	// If "Type your own" is selected, check that custom text is non-empty
	if selected[0] == "Type your own answer" {
		return strings.TrimSpace(q.customInput.Value()) != ""
	}
	
	return true
}

func (q QuestionView) View() string {
	var b strings.Builder
	
	// Question counter
	counterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	b.WriteString(counterStyle.Render(
		fmt.Sprintf("Question %d of %d", q.currentIndex+1, len(q.questions))))
	b.WriteString("\n\n")
	
	currentQ := q.questions[q.currentIndex]
	
	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cdd6f4"))
	b.WriteString(headerStyle.Render(currentQ.Header))
	b.WriteString("\n\n")
	
	// Question text
	b.WriteString(currentQ.Question)
	b.WriteString("\n\n")
	
	// Options (using QuestionOptions component)
	b.WriteString(q.optionSelector.View())
	
	// Custom input (if visible)
	if q.showCustom {
		b.WriteString("\n")
		b.WriteString(q.customInput.View())
		b.WriteString("\n")
	}
	
	// Navigation buttons
	b.WriteString("\n")
	var buttons []wizard.Button
	
	// Back button (only show if not first question)
	if q.currentIndex > 0 {
		backState := wizard.ButtonNormal
		if q.focusIndex == q.backButtonFocusIndex() {
			backState = wizard.ButtonFocused
		}
		buttons = append(buttons, wizard.Button{
			Label: "← Back",
			State: backState,
		})
	}
	
	// Next/Submit button
	nextLabel := "Next →"
	if q.currentIndex == len(q.questions)-1 {
		nextLabel = "Submit"
	}
	nextState := wizard.ButtonNormal
	if q.focusIndex == q.nextButtonFocusIndex() {
		nextState = wizard.ButtonFocused
	}
	buttons = append(buttons, wizard.Button{
		Label: nextLabel,
		State: nextState,
	})
	
	buttonBar := wizard.NewButtonBar(buttons)
	b.WriteString(buttonBar.Render())
	
	return b.String()
}
```

This complete example shows:
- **Option selection** with keyboard navigation (up/down/j/k, space to toggle)
- **Focus management** across options, custom input, and buttons
- **Answer persistence** via restoreAnswer() and saveCurrentAnswer()
- **Tab navigation** between all focusable components
- **Validation** before navigation/submission
- **Integration** with existing ButtonBar component from wizard package
```

#### 4. Radio Button & Multi-Select Pattern (Based on Bubbles List Component)
**Complete working example for question options with keyboard navigation:**

```go
// OptionItem represents a single question option
type OptionItem struct {
	label       string
	description string
	selected    bool
}

// QuestionOptions manages option selection with keyboard navigation
type QuestionOptions struct {
	items      []OptionItem
	cursor     int  // Currently focused option
	multiSelect bool // Allow multiple selections
	focused    bool
}

func NewQuestionOptions(opts []OptionItem, multiSelect bool) QuestionOptions {
	return QuestionOptions{
		items:       opts,
		cursor:      0,
		multiSelect: multiSelect,
		focused:     true,
	}
}

// Keyboard navigation methods
func (q *QuestionOptions) CursorUp() {
	if q.cursor > 0 {
		q.cursor--
	}
}

func (q *QuestionOptions) CursorDown() {
	if q.cursor < len(q.items)-1 {
		q.cursor++
	}
}

func (q *QuestionOptions) Toggle() {
	isCustomOption := q.cursor == len(q.items)-1 // Last item is "Type your own answer"
	
	if q.multiSelect {
		if isCustomOption {
			// "Type your own" is mutually exclusive in multi-select
			for i := range q.items {
				q.items[i].selected = false
			}
			q.items[q.cursor].selected = true
		} else {
			// Selecting normal option: deselect "Type your own" and toggle current
			q.items[len(q.items)-1].selected = false
			q.items[q.cursor].selected = !q.items[q.cursor].selected
		}
	} else {
		// Single-select: unselect all, select current
		for i := range q.items {
			q.items[i].selected = false
		}
		q.items[q.cursor].selected = true
	}
}

func (q QuestionOptions) SelectedIndices() []int {
	var indices []int
	for i, item := range q.items {
		if item.selected {
			indices = append(indices, i)
		}
	}
	return indices
}

func (q QuestionOptions) SelectedLabels() []string {
	var labels []string
	for _, item := range q.items {
		if item.selected {
			labels = append(labels, item.label)
		}
	}
	return labels
}

func (q *QuestionOptions) Focus() { q.focused = true }
func (q *QuestionOptions) Blur()  { q.focused = false }

func (q QuestionOptions) Update(msg tea.Msg) (QuestionOptions, tea.Cmd) {
	if !q.focused {
		return q, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			q.CursorUp()
		case "down", "j":
			q.CursorDown()
		case " ", "enter":
			q.Toggle()
		}
	}
	return q, nil
}

func (q QuestionOptions) View() string {
	var b strings.Builder
	
	for i, item := range q.items {
		// Determine indicator based on selection type
		var indicator string
		if q.multiSelect {
			indicator = "☐" // Checkbox
			if item.selected {
				indicator = "☑"
			}
		} else {
			indicator = "○" // Radio button
			if item.selected {
				indicator = "●"
			}
		}
		
		// Show cursor for focused item
		cursor := "  "
		if i == q.cursor && q.focused {
			cursor = "▶ "
		}
		
		// Style based on focus/selection
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
		if i == q.cursor && q.focused {
			style = style.Foreground(lipgloss.Color("#b4befe")).Bold(true)
		}
		
		line := fmt.Sprintf("%s%s %s", cursor, indicator, item.label)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		
		// Show description with indent
		if item.description != "" {
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
			b.WriteString(descStyle.Render(fmt.Sprintf("      %s", item.description)))
			b.WriteString("\n")
		}
	}
	
	return b.String()
}
```

#### 5. Wizard Step Management Pattern (internal/tui/specwizard/wizard.go)
```go
type WizardModel struct {
	step int // 0=title, 1=description, 2=model, 3=agent, 4=completion
	
	// Step data
	title       string
	description string
	model       string
	specPath    string
	
	// Step components (reuse from build wizard)
	titleInput      textinput.Model
	descriptionArea textarea.Model  
	modelSelector   *wizard.ModelSelectorStep // REUSE
	agentPhase      *AgentPhaseStep
	completion      *CompletionStep
	buttonBar       *wizard.ButtonBar // REUSE
	
	// MCP server
	mcpServer *specmcp.Server
	
	// Agent
	agent *agent.Runner
	
	width  int
	height int
}

func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			// Handle tab navigation similar to build wizard
			// Cycle between step content and button bar
			return m, nil
		}
	
	case wizard.ModelSelectedMsg:
		// Model selected, advance to agent phase
		m.model = msg.ModelID
		m.step = 3
		return m, m.startAgentPhase()
	
	case specmcp.QuestionRequest:
		// Question received from MCP server
		// Forward to agent phase component
		return m, m.agentPhase.ShowQuestion(msg)
	}
	
	// Forward to current step
	return m.updateCurrentStep(msg)
}
```

### Config Changes
Add to `internal/config/config.go`:
```go
type Config struct {
    // ... existing fields
    SpecDir string `mapstructure:"spec_dir"`
}
```
Default: `./specs`, env: `ITERATR_SPEC_DIR`

## UI Mockup

### Name Step
```
+- Spec Wizard - Step 1 of 3: Name --------------------+
|                                                      |
|  Enter spec name (lowercase, hyphens only):          |
|  +------------------------------------------------+  |
|  | my-feature-name                                |  |
|  +------------------------------------------------+  |
|                                                      |
|                          [ Cancel ]  [ Next -> ]     |
+------------------------------------------------------+
```

### Description Step
```
+- Spec Wizard - Step 2 of 3: Description -------------+
|                                                      |
|  Describe the feature in detail:                     |
|  +------------------------------------------------+  |
|  | I want to add a new subcommand `spec` that     |  |
|  | first shows a wizard similar to the wizard in  |  |
|  | the `build` subcommand. It asks for a name     |  |
|  | then a description...                          |  |
|  +------------------------------------------------+  |
|                                                      |
|                          [ <- Back ]  [ Next -> ]    |
+------------------------------------------------------+
```

### Agent Phase (Thinking)
```
+- Spec Wizard - Interview ----------------------------+
|                                                      |
|                                                      |
|          [spinner] Agent is analyzing requirements...|
|                                                      |
|                                                      |
|                          [ Cancel ]                  |
+------------------------------------------------------+
```

### Agent Phase (Question)
```
+- Spec Wizard - Interview ----------------------------+
|                                                      |
|  Error Handling                                      |
|                                                      |
|  What should happen if the API request fails?        |
|                                                      |
|  > * Retry with exponential backoff                  |
|      Automatic retry up to 3 times                   |
|    o Show error and let user retry                   |
|      Display error modal with retry button           |
|    o Fail silently with fallback                     |
|      Use cached data if available                    |
|    o Type your own answer...                         |
|                                                      |
|                          [ Submit ]                  |
+------------------------------------------------------+
```

### Completion
```
+- Spec Wizard - Complete -----------------------------+
|                                                      |
|  [check] Spec saved to specs/my-feature-name.md      |
|  [check] Updated specs/README.md                     |
|                                                      |
|                                                      |
|          [ View ]  [ Start Build ]  [ Exit ]         |
+------------------------------------------------------+
```

## Tasks

### Tracer Bullet: End-to-End Skeleton
**Goal**: Get minimal working flow from title input -> agent prompt -> spec saved, proving all pieces connect.

- [ ] Create `cmd/iteratr/spec.go` with RunE calling wizard.Run()
- [ ] Create `internal/tui/specwizard/wizard.go` with hardcoded 1-step flow (title input only)
- [ ] Create `internal/specmcp/server.go` with minimal Start() returning port
- [ ] Create `internal/specmcp/handlers.go` with finish-spec handler that prints content and saves to file
- [ ] Spawn opencode acp, send initialize + session/new with mcpServers array (copy pattern from internal/agent/acp.go)
- [ ] Send hardcoded prompt via session/prompt request
- [ ] Verify agent calls finish-spec tool and content appears in handler
- [ ] Save content to `./specs/test-{timestamp}.md` to avoid conflicts
- [ ] Run: `iteratr spec` -> type title -> see agent run -> see file created
- [ ] Verify workflow end-to-end, then clean up test files
- [ ] Proceed to full implementation with all wizard steps

### 1. Configuration
- [ ] Add `spec_dir` field to Config struct with default `./specs`, env: ITERATR_SPEC_DIR

### 2. MCP Server Foundation
- [ ] Implement Server struct with question/answer channels (see code example above)
- [ ] Implement Start() with random port selection (copy pattern from internal/mcpserver)
- [ ] Implement Stop() with clean HTTP server shutdown
- [ ] Add QuestionChan() and SendAnswers() methods for UI communication

### 3. MCP Tool Registration
- [ ] Register ask-questions tool with schema matching OpenCode question tool
- [ ] Register finish-spec tool with content parameter (single string)

### 4. Ask Questions Handler
- [ ] Parse questions array from MCP request arguments
- [ ] Send QuestionRequest to questionCh with result channel
- [ ] Block waiting for answers from answerCh
- [ ] Return answers as JSON array to agent
- [ ] Handle context cancellation

### 5. Finish Spec Handler
- [ ] Extract content parameter (validate non-empty)
- [ ] Use s.specTitle from server state (NOT parsed from content)
- [ ] Slugify title (lowercase, spaces->hyphens, transliterate accents: github.com/gosimple/slug)
- [ ] Validate content has required sections:
```go
func validateSpecContent(content string) error {
    lower := strings.ToLower(content)
    if !strings.Contains(lower, "## overview") {
        return fmt.Errorf("spec is missing '## Overview' section")
    }
    if !strings.Contains(lower, "## tasks") {
        return fmt.Errorf("spec is missing '## Tasks' section")
    }
    return nil
}
```
- [ ] Check if file exists at {spec_dir}/{slug}.md
  - If exists: return error message asking agent to confirm or choose new name
- [ ] Write content to file with proper permissions (0644)
- [ ] Call updateREADME(specDir, slug, title, description) helper (see task 6)
- [ ] Return success with file path

### 6. README Update Logic
- [ ] Implement updateREADME(specDir, slug, title, description string) function
- [ ] Check if README.md exists, create if missing with header + table
- [ ] Look for `<!-- SPECS -->` marker
- [ ] If found: insert new row after marker
- [ ] If not found: append marker + table header + new row
- [ ] Table format: `| [Title](slug.md) | Description | Date |`
  - Title: Link to spec file
  - Description: First line of wizard description (up to 100 chars, trim if needed)
  - Date: time.Now().Format("2006-01-02")
- [ ] Handle edge cases: description with newlines (use first line only)

### 7. Wizard Framework
- [ ] Create wizard.go with step enum (title, description, model, agent, completion)
- [ ] Implement Init() initializing title step
- [ ] Implement Update() with step navigation (reuse patterns from build wizard)
- [ ] Implement View() with modal rendering (copy from build wizard)
- [ ] Add buttonBar field reusing wizard.ButtonBar component
- [ ] Implement tab focus cycling between step content and button bar

### 8. Title Input Step
- [ ] Create title_step.go with textinput.Model
- [ ] Add real-time slug validation using regex pattern:
```go
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateSlug(s string) error {
    if s == "" {
        return fmt.Errorf("title cannot be empty")
    }
    if !slugPattern.MatchString(s) {
        return fmt.Errorf("use only lowercase letters, numbers, and hyphens (no spaces, cannot start/end with hyphen)")
    }
    if len(s) > 100 {
        return fmt.Errorf("title too long (max 100 characters)")
    }
    return nil
}
```
- [ ] Show validation error if invalid characters typed
- [ ] Render with hint: "lowercase letters, numbers, hyphens only"

### 9. Description Textarea Step
- [ ] Create description_step.go with textarea.Model (copy pattern from template_editor.go)
- [ ] Set hint: "provide as much detail as possible"
- [ ] Support multi-line editing with scrolling
- [ ] Allow empty description (optional field)

### 10. Model Selector Integration
- [ ] Import wizard.NewModelSelectorStep() in wizard.go
- [ ] Instantiate as step 2 (after description step)
- [ ] Forward Update() messages when on model step
- [ ] Handle wizard.ModelSelectedMsg to advance to agent phase
- [ ] Pass wizard title and description to agent phase for MCP server initialization

### 11. Agent Phase Component
- [ ] Create agent_phase.go with spinner, status text, question state management
- [ ] Add state fields:
  - questions []Question - all questions received from agent
  - answers []QuestionAnswer - parallel array storing answer for each question
  - currentQuestionIndex int - which question is being displayed
  - waitingForAgent bool - true when showing spinner, false when showing question
- [ ] Implement startAgentPhase() spawning opencode acp with stateless runner (see research/opencode-acp-protocol.md)
- [ ] Start MCP server with New(specTitle, specDir), get port from Start()
- [ ] Spawn opencode process via exec.Command("opencode", "acp")
- [ ] Send initialize request over stdin/stdout JSON-RPC (see internal/agent/acp.go)
- [ ] Send session/new request with mcpServers array containing iteratr-spec HTTP server URL
  - Example: `{"type": "http", "name": "iteratr-spec", "url": "http://localhost:{port}/mcp"}`
- [ ] Construct agent prompt using buildSpecPrompt(title, description):
```go
func buildSpecPrompt(title, description string) string {
    specFormat := `# [Title]

## Overview
Brief description of the feature

## User Story
Who benefits and why

## Requirements
Detailed requirements

## Technical Implementation
Implementation details

## Tasks
Byte-sized implementation tasks

## Out of Scope
What's not included in v1`

    return fmt.Sprintf(`You are helping create a feature specification.

Feature: %s
Description: %s

Follow the user instructions and interview me in detail using the ask-questions 
tool about literally anything: technical implementation, UI & UX, concerns, 
tradeoffs, edge cases, dependencies, testing, etc. Be very in-depth and continue 
interviewing me continually until you have enough information. Then write the 
complete spec using the finish-spec tool.

The spec MUST follow this format:

%s

Make the spec extremely concise. Sacrifice grammar for the sake of concision.`, 
        title, description, specFormat)
}
```
- [ ] Send prompt via session/prompt request
- [ ] Handle agent_message_chunk notifications (update spinner status text)
- [ ] Handle agent_thought_chunk notifications (optionally show thinking state)
- [ ] Listen on mcpServer.QuestionChan() for question requests:
  - Receive all questions at once, store in questions array
  - Initialize answers array with empty QuestionAnswer structs (same length as questions)
  - Set currentQuestionIndex = 0, show first question
- [ ] Wait for session/prompt response with stopReason

### 12. Question View Component
- [ ] Create question_view.go with state for current question + all answers
- [ ] Constructor: NewQuestionView(questions []Question, answers []QuestionAnswer, currentIndex int)
- [ ] Implement FocusableComponent pattern (see btca response above) with SetFocus/GetFocus methods
- [ ] Implement Update() with full tab navigation:
  - Tab: cycle through options list -> custom input (if "Type your own" selected) -> Back button -> Next/Submit button -> wrap
  - Shift+Tab: reverse cycle
  - Up/Down/j/k: navigate options when option list focused
  - Space/Enter: toggle option selection or activate focused widget
- [ ] Track focusIndex (0=options, 1=custom input, 2=back button, 3=next/submit button) and selectedIndices []int
- [ ] Auto-append "Type your own answer" option at end (matching OpenCode behavior)
- [ ] When "Type your own" selected, show textinput.Model below options
- [ ] Focus custom input automatically when tabbing past options if it's selected
- [ ] On view initialization, restore previous answer from answers[currentIndex]:
  - Parse answer string to determine which option(s) were selected
  - Set selectedIndices accordingly
  - If custom text, populate textinput with previous value
- [ ] Navigation buttons (reuse ButtonBar pattern):
  - Back button: enabled if currentIndex > 0, returns PrevQuestionMsg
  - Next button: shown if currentIndex < len(questions)-1, returns NextQuestionMsg
  - Submit button: shown if currentIndex == len(questions)-1, validates and returns SubmitAnswersMsg
- [ ] On Next/Back: save current answer to answers[currentIndex] before navigating
- [ ] Validate answer on Next/Submit: at least one option selected or custom text non-empty
- [ ] Show error if validation fails, keep focus on question
- [ ] Render with visual focus indicators:
  - Options: "▶" prefix for focused, "○"/"●" for unselected/selected
  - Custom input: use textinput focused styles
  - Buttons: bold + highlighted background when focused (copy from button_bar.go)
- [ ] Show "Question X of Y" counter at top
- [ ] Support multi-select with []int tracking multiple selected indices

### 13. Question Navigation & Answer Submission
- [ ] Handle NextQuestionMsg in agent_phase.go:
  - Validate current answer (selectedIndices not empty or custom text non-empty)
  - Save answer to answers[currentQuestionIndex]
  - Increment currentQuestionIndex
  - Re-render question view with new index
- [ ] Handle PrevQuestionMsg in agent_phase.go:
  - Save current answer to answers[currentQuestionIndex] (no validation needed)
  - Decrement currentQuestionIndex
  - Re-render question view with new index (loads previous answer automatically)
- [ ] Handle SubmitAnswersMsg in agent_phase.go:
  - Validate all questions answered (check each QuestionAnswer has non-empty Value)
  - If validation fails, show error and navigate to first unanswered question
  - Format answers for MCP: build []interface{} where each element is:
    - string for single-select (answer.Value as string)
    - []string for multi-select (answer.Value as []string)
  - Send formatted answers via mcpServer.SendAnswers()
  - Set waitingForAgent = true, return to spinner view while agent processes

Example answer formatting:
```go
func formatAnswersForMCP(answers []QuestionAnswer) []interface{} {
	result := make([]interface{}, len(answers))
	for i, ans := range answers {
		if ans.IsMulti {
			result[i] = ans.Value // Already []string
		} else {
			result[i] = ans.Value // Already string
		}
	}
	return result
}
```

### 14. Agent Completion Handling
- [ ] Handle agent stop message (end of session)
- [ ] Shut down MCP server via Stop()
- [ ] Verify spec file was saved (check file exists)
- [ ] Advance to completion step with spec path

### 15. Completion Step
- [ ] Create completion_step.go with success message, file path display
- [ ] Add three buttons: View | Start Build | Exit (reuse ButtonBar with 3 buttons)
- [ ] View button: use `github.com/charmbracelet/x/editor` package with `tea.ExecProcess` (same pattern as template_editor.go):
```go
import "github.com/charmbracelet/x/editor"

func (c *CompletionStep) openInEditor() tea.Cmd {
    // editor.Command checks $EDITOR, $VISUAL, and falls back to sensible defaults
    cmd, err := editor.Command("iteratr", c.specPath)
    if err != nil {
        // No editor available - show path instead
        return func() tea.Msg {
            return ShowMessageMsg{msg: fmt.Sprintf("Spec saved to: %s", c.specPath)}
        }
    }
    
    // Execute editor - suspends TUI, restores after editor exits
    return tea.ExecProcess(cmd, func(err error) tea.Msg {
        if err != nil {
            return ShowMessageMsg{msg: fmt.Sprintf("Editor error: %v\nSpec saved to: %s", err, c.specPath)}
        }
        return EditorClosedMsg{}
    })
}
```
- [ ] Only show View button if editor is available (check `os.Getenv("EDITOR") != ""`)
- [ ] Start Build button: exec `iteratr build --spec {path}` directly, exit wizard
- [ ] Exit button: return tea.Quit
- [ ] Implement tab navigation between buttons

### 16. Cancellation & Error Handling
- [ ] Add ESC handler in question view:
  - If on first question: show cancel confirmation modal
  - If on question 2+: treat as Back button (go to previous question)
- [ ] Add ESC handler during spinner/agent thinking: show cancel confirmation modal
- [ ] Confirmation modal: "Cancel spec creation?" with Yes/No buttons (reuse ButtonBar)
- [ ] On Yes: send session/cancel notification, stop MCP server, discard progress, exit wizard
- [ ] Handle agent early termination (ends without calling finish-spec): show error, exit
- [ ] Handle MCP server start failure: show error modal, exit wizard
- [ ] Handle opencode not installed: show error with install link, exit wizard
- [ ] Handle finish-spec validation failure: error is returned to agent, agent can retry

### 17. Integration & Polish
- [ ] Wire wizard.Run() into cmd/iteratr/spec.go
- [ ] Add wizard to root command
- [ ] Ensure spec_dir created if doesn't exist
- [ ] Manual E2E test: create spec from scratch, verify file + README updated
- [ ] Test overwrite flow: create same spec twice, agent should handle error and ask for new name
- [ ] Test cancellation: press ESC during questions, confirm and exit cleanly
- [ ] Test all tab navigation paths in question view

## Out of Scope

- CLI flags for non-interactive use (always wizard)
- Session persistence for spec interviews
- Resuming interrupted spec sessions
- Editing existing specs through wizard
- Multiple spec generation in single session

## Open Questions

1. Should finish-spec support a `confirmed_overwrite: bool` param, or require agent to call with different name?
2. Should README migration from old format be a separate command?
3. Future: support for templates (different spec formats for different project types)?
