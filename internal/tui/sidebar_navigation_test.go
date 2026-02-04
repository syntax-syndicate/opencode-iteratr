package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/iteratr/internal/session"
)

// TestSidebar_TaskNavigation_Down tests navigating down through tasks with j/down keys
func TestSidebar_TaskNavigation_Down(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create state with 3 tasks
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "in_progress", Priority: 1},
			"task3": {ID: "task3", Content: "Third task", Status: "completed", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Initial cursor should be at 0
	if sidebar.cursor != 0 {
		t.Errorf("Initial cursor: got %d, want 0", sidebar.cursor)
	}

	// Press 'j' to move down
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != 1 {
		t.Errorf("After first 'j': cursor got %d, want 1", sidebar.cursor)
	}

	// Press 'down' arrow to move down again
	sidebar.Update(tea.KeyPressMsg{Text: "down"})
	if sidebar.cursor != 2 {
		t.Errorf("After 'down': cursor got %d, want 2", sidebar.cursor)
	}

	// Trying to go past last item should keep cursor at last position
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != 2 {
		t.Errorf("After 'j' at end: cursor got %d, want 2", sidebar.cursor)
	}
}

// TestSidebar_TaskNavigation_Up tests navigating up through tasks with k/up keys
func TestSidebar_TaskNavigation_Up(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create state with 3 tasks
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "in_progress", Priority: 1},
			"task3": {ID: "task3", Content: "Third task", Status: "completed", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Move to last task
	sidebar.cursor = 2

	// Press 'k' to move up
	sidebar.Update(tea.KeyPressMsg{Text: "k"})
	if sidebar.cursor != 1 {
		t.Errorf("After first 'k': cursor got %d, want 1", sidebar.cursor)
	}

	// Press 'up' arrow to move up again
	sidebar.Update(tea.KeyPressMsg{Text: "up"})
	if sidebar.cursor != 0 {
		t.Errorf("After 'up': cursor got %d, want 0", sidebar.cursor)
	}

	// Trying to go past first item should keep cursor at 0
	sidebar.Update(tea.KeyPressMsg{Text: "k"})
	if sidebar.cursor != 0 {
		t.Errorf("After 'k' at start: cursor got %d, want 0", sidebar.cursor)
	}
}

// TestSidebar_TaskNavigation_EnterOpensModal tests that pressing Enter on a task opens the modal
func TestSidebar_TaskNavigation_EnterOpensModal(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create state with 2 tasks
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "in_progress", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Navigate to second task
	sidebar.cursor = 1
	sidebar.updateContent()

	// Press Enter
	cmd := sidebar.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("Enter should return command to open modal")
	}

	// Execute command to get message
	msg := cmd()
	openMsg, ok := msg.(OpenTaskModalMsg)
	if !ok {
		t.Fatalf("Expected OpenTaskModalMsg, got %T", msg)
	}

	// Verify correct task is selected
	if openMsg.Task.ID != "task2" {
		t.Errorf("OpenTaskModalMsg task ID: got %s, want task2", openMsg.Task.ID)
	}
	if openMsg.Task.Content != "Second task" {
		t.Errorf("OpenTaskModalMsg task content: got %s, want 'Second task'", openMsg.Task.Content)
	}
}

// TestSidebar_TaskNavigation_NoFocus tests that navigation doesn't work when not focused
func TestSidebar_TaskNavigation_NoFocus(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = false // Not focused

	// Create state with tasks
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "in_progress", Priority: 1},
		},
	}
	sidebar.SetState(state)

	initialCursor := sidebar.cursor

	// Try to navigate - should be ignored
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != initialCursor {
		t.Errorf("Cursor should not change when not focused: got %d, want %d", sidebar.cursor, initialCursor)
	}

	sidebar.Update(tea.KeyPressMsg{Text: "enter"})
	// Should return nil since not focused
	cmd := sidebar.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd != nil {
		t.Error("Enter should not work when sidebar not focused")
	}
}

// TestSidebar_TaskNavigation_EmptyList tests navigation with no tasks
func TestSidebar_TaskNavigation_EmptyList(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create empty state
	state := &session.State{
		Tasks: map[string]*session.Task{},
	}
	sidebar.SetState(state)

	// Try navigation on empty list
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != 0 {
		t.Errorf("Cursor on empty list: got %d, want 0", sidebar.cursor)
	}

	// Try enter on empty list
	cmd := sidebar.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd != nil {
		t.Error("Enter on empty list should return nil")
	}
}

// TestSidebar_TaskNavigation_SingleTask tests navigation with only one task
func TestSidebar_TaskNavigation_SingleTask(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create state with single task
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "Only task", Status: "remaining", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Cursor should stay at 0
	if sidebar.cursor != 0 {
		t.Errorf("Initial cursor: got %d, want 0", sidebar.cursor)
	}

	// Try moving down - should stay at 0
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != 0 {
		t.Errorf("After 'j' with single task: cursor got %d, want 0", sidebar.cursor)
	}

	// Try moving up - should stay at 0
	sidebar.Update(tea.KeyPressMsg{Text: "k"})
	if sidebar.cursor != 0 {
		t.Errorf("After 'k' with single task: cursor got %d, want 0", sidebar.cursor)
	}

	// Enter should still work
	cmd := sidebar.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Error("Enter on single task should return command")
	}
}

// TestSidebar_TaskNavigation_CursorPersistence tests cursor position maintained across state updates
func TestSidebar_TaskNavigation_CursorPersistence(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create initial state
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "remaining", Priority: 1},
			"task3": {ID: "task3", Content: "Third task", Status: "remaining", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Navigate to second task
	sidebar.Update(tea.KeyPressMsg{Text: "j"})
	if sidebar.cursor != 1 {
		t.Fatalf("Cursor should be at 1, got %d", sidebar.cursor)
	}

	// Update state (task status change)
	updatedState := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "in_progress", Priority: 1},
			"task3": {ID: "task3", Content: "Third task", Status: "remaining", Priority: 1},
		},
	}
	sidebar.SetState(updatedState)

	// Cursor should remain at position 1
	if sidebar.cursor != 1 {
		t.Errorf("Cursor should persist across state updates: got %d, want 1", sidebar.cursor)
	}
}

// TestSidebar_TaskNavigation_BoundaryConditions tests edge cases
func TestSidebar_TaskNavigation_BoundaryConditions(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)
	sidebar.tasksFocused = true

	// Create state with tasks
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task1": {ID: "task1", Content: "First task", Status: "remaining", Priority: 1},
			"task2": {ID: "task2", Content: "Second task", Status: "remaining", Priority: 1},
			"task3": {ID: "task3", Content: "Third task", Status: "remaining", Priority: 1},
		},
	}
	sidebar.SetState(state)

	// Test multiple down presses beyond boundary
	for i := 0; i < 10; i++ {
		sidebar.Update(tea.KeyPressMsg{Text: "j"})
	}
	if sidebar.cursor != 2 {
		t.Errorf("Cursor after multiple down presses: got %d, want 2", sidebar.cursor)
	}

	// Test multiple up presses beyond boundary
	for i := 0; i < 10; i++ {
		sidebar.Update(tea.KeyPressMsg{Text: "k"})
	}
	if sidebar.cursor != 0 {
		t.Errorf("Cursor after multiple up presses: got %d, want 0", sidebar.cursor)
	}
}

// TestSidebar_TaskNavigation_ScrollToItem tests that navigation triggers scroll
func TestSidebar_TaskNavigation_ScrollToItem(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 10) // Small height to force scrolling
	sidebar.tasksFocused = true

	// Create state with many tasks
	tasks := make(map[string]*session.Task)
	for i := 0; i < 20; i++ {
		taskID := string(rune('a' + i))
		tasks[taskID] = &session.Task{
			ID:       taskID,
			Content:  "Task " + taskID,
			Status:   "remaining",
			Priority: 1,
		}
	}
	state := &session.State{Tasks: tasks}
	sidebar.SetState(state)

	// Navigate down several times
	for i := 0; i < 10; i++ {
		sidebar.Update(tea.KeyPressMsg{Text: "j"})
	}

	// Cursor should be at position 10
	if sidebar.cursor != 10 {
		t.Errorf("Cursor after navigation: got %d, want 10", sidebar.cursor)
	}

	// Verify scroll list is tracking cursor position
	// (ScrollList.ScrollToItem should have been called)
	// We can't directly test scroll position without accessing internals,
	// but we verify the cursor moved correctly
}

// TestSidebar_TaskOrdering tests that tasks are displayed in ID alphabetical order
func TestSidebar_TaskOrdering(t *testing.T) {
	sidebar := NewSidebar()
	sidebar.SetSize(40, 30)

	// Create tasks with different IDs (getTasks sorts by ID)
	state := &session.State{
		Tasks: map[string]*session.Task{
			"task-c": {ID: "task-c", Content: "Third alphabetically", Status: "remaining", Priority: 1},
			"task-a": {ID: "task-a", Content: "First alphabetically", Status: "remaining", Priority: 3},
			"task-b": {ID: "task-b", Content: "Second alphabetically", Status: "remaining", Priority: 0},
		},
	}
	sidebar.SetState(state)

	// Get ordered tasks
	tasks := sidebar.getTasks()

	// Verify tasks are ordered by ID (alphabetical)
	if len(tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(tasks))
	}

	// Task order should be alphabetical: task-a, task-b, task-c
	if tasks[0].ID != "task-a" {
		t.Errorf("First task should be task-a, got %s", tasks[0].ID)
	}
	if tasks[1].ID != "task-b" {
		t.Errorf("Second task should be task-b, got %s", tasks[1].ID)
	}
	if tasks[2].ID != "task-c" {
		t.Errorf("Third task should be task-c, got %s", tasks[2].ID)
	}
}
