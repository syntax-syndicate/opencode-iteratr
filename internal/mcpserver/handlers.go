package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/iteratr/internal/session"
	"github.com/mark3labs/mcp-go/mcp"
)

// validateInProgress checks whether a task can be set to in_progress.
// Returns a non-empty guidance message if the transition is not allowed.
// Rule 1: Only one task can be in progress at a time.
// Rule 2: No new tasks can be marked in progress until a new iteration starts.
func validateInProgress(state *session.State, currentIteration int) string {
	// Rule 1: Check if any task is already in progress
	for _, task := range state.Tasks {
		if task.Status == "in_progress" {
			return fmt.Sprintf(
				"Only one task can be in progress at a time. Task %s (%q) is currently in progress. "+
					"Complete or update it before starting another task.",
				task.ID, task.Content,
			)
		}
	}

	// Rule 2: Check if a task was already started during this iteration
	if len(state.Iterations) > 0 {
		currentIter := state.Iterations[len(state.Iterations)-1]
		if currentIter.Number == currentIteration && currentIter.TaskStarted {
			return "A task was already started during this iteration. " +
				"Record your iteration summary and wait for the next iteration before starting a new task."
		}
	}

	return ""
}

// handleTaskAdd adds one or more tasks to the session.
func (s *Server) handleTaskAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultText("error: no arguments provided"), nil
	}

	// Extract tasks array
	tasksRaw, ok := args["tasks"]
	if !ok {
		return mcp.NewToolResultText("error: missing 'tasks' parameter"), nil
	}

	// Type assert to []any (mcp-go returns arrays as []any)
	tasksArray, ok := tasksRaw.([]any)
	if !ok {
		return mcp.NewToolResultText("error: 'tasks' is not an array"), nil
	}

	if len(tasksArray) == 0 {
		return mcp.NewToolResultText("error: at least one task is required"), nil
	}

	// Parse each task into TaskAddParams
	taskParams := make([]session.TaskAddParams, 0, len(tasksArray))
	for i, taskRaw := range tasksArray {
		// Convert to map[string]any
		taskMap, ok := taskRaw.(map[string]any)
		if !ok {
			return mcp.NewToolResultText(fmt.Sprintf("error: task %d is not an object", i)), nil
		}

		// Extract content (required)
		content, ok := taskMap["content"].(string)
		if !ok || content == "" {
			return mcp.NewToolResultText(fmt.Sprintf("error: task %d missing or empty 'content' field", i)), nil
		}

		// Extract optional status
		status := ""
		if statusVal, ok := taskMap["status"].(string); ok {
			status = statusVal
		}

		// Extract optional priority (JSON numbers come as float64)
		priority := 0
		if priorityVal, ok := taskMap["priority"].(float64); ok {
			priority = int(priorityVal)
		}

		taskParams = append(taskParams, session.TaskAddParams{
			Content:  content,
			Status:   status,
			Priority: priority,
			// Iteration will be set by store based on current iteration
		})
	}

	// Validate in_progress constraints if any task requests that status
	inProgressCount := 0
	for _, tp := range taskParams {
		if tp.Status == "in_progress" {
			inProgressCount++
		}
	}
	if inProgressCount > 1 {
		return mcp.NewToolResultText(
			"Only one task can be in progress at a time. This batch contains multiple tasks with in_progress status.",
		), nil
	}
	if inProgressCount == 1 {
		state, err := s.store.LoadState(ctx, s.sessName)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("error: failed to load state: %v", err)), nil
		}
		currentIteration := 0
		if len(state.Iterations) > 0 {
			currentIteration = state.Iterations[len(state.Iterations)-1].Number
		}
		if msg := validateInProgress(state, currentIteration); msg != "" {
			return mcp.NewToolResultText(msg), nil
		}
	}

	// Call TaskBatchAdd
	tasks, err := s.store.TaskBatchAdd(ctx, s.sessName, taskParams)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	// Return success message with task IDs
	result := fmt.Sprintf("Added %d task(s):", len(tasks))
	for _, task := range tasks {
		result += fmt.Sprintf("\n  %s: %s", task.ID, task.Content)
	}

	return mcp.NewToolResultText(result), nil
}

// handleTaskUpdate updates a task's status, priority, or dependencies.
func (s *Server) handleTaskUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultText("error: no arguments provided"), nil
	}

	// Extract required id parameter
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return mcp.NewToolResultText("error: missing or invalid 'id' parameter"), nil
	}

	// Load state to get current iteration number
	state, err := s.store.LoadState(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: failed to load state: %v", err)), nil
	}

	// Get current iteration number (last iteration in slice)
	currentIteration := 0
	if len(state.Iterations) > 0 {
		currentIteration = state.Iterations[len(state.Iterations)-1].Number
	}

	// Track what we updated for the success message
	updated := []string{}

	// Update status if provided
	if status, ok := args["status"].(string); ok && status != "" {
		// Validate in_progress transitions
		if status == "in_progress" {
			if msg := validateInProgress(state, currentIteration); msg != "" {
				return mcp.NewToolResultText(msg), nil
			}
		}

		err := s.store.TaskStatus(ctx, s.sessName, session.TaskStatusParams{
			ID:        id,
			Status:    status,
			Iteration: currentIteration,
		})
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("error: failed to update status: %v", err)), nil
		}
		updated = append(updated, fmt.Sprintf("status=%s", status))
	}

	// Update priority if provided (JSON numbers come as float64)
	if priorityVal, ok := args["priority"]; ok {
		var priority int
		switch v := priorityVal.(type) {
		case float64:
			priority = int(v)
		case int:
			priority = v
		default:
			return mcp.NewToolResultText("error: 'priority' must be a number"), nil
		}

		err := s.store.TaskPriority(ctx, s.sessName, session.TaskPriorityParams{
			ID:        id,
			Priority:  priority,
			Iteration: currentIteration,
		})
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("error: failed to update priority: %v", err)), nil
		}
		updated = append(updated, fmt.Sprintf("priority=%d", priority))
	}

	// Update depends_on if provided
	if dependsOn, ok := args["depends_on"].(string); ok && dependsOn != "" {
		err := s.store.TaskDepends(ctx, s.sessName, session.TaskDependsParams{
			ID:        id,
			DependsOn: dependsOn,
			Iteration: currentIteration,
		})
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("error: failed to update dependency: %v", err)), nil
		}
		updated = append(updated, fmt.Sprintf("depends_on=%s", dependsOn))
	}

	// Check if anything was actually updated
	if len(updated) == 0 {
		return mcp.NewToolResultText("error: no valid update parameters provided (status, priority, or depends_on required)"), nil
	}

	// Return success message
	result := fmt.Sprintf("Updated task %s: %s", id, updated[0])
	for i := 1; i < len(updated); i++ {
		result += fmt.Sprintf(", %s", updated[i])
	}

	return mcp.NewToolResultText(result), nil
}

// handleTaskList returns all tasks grouped by status.
func (s *Server) handleTaskList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Call TaskList
	result, err := s.store.TaskList(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	// Format output matching CLI format
	var lines []string
	formatTasks := func(status string, tasks []*session.Task) {
		if len(tasks) == 0 {
			return
		}
		// Capitalize first letter of status and replace underscores
		statusLabel := strings.ReplaceAll(status, "_", " ")
		if len(statusLabel) > 0 {
			statusLabel = strings.ToUpper(statusLabel[:1]) + statusLabel[1:]
		}
		lines = append(lines, fmt.Sprintf("%s:", statusLabel))
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("  [%s] %s", t.ID, t.Content))
		}
	}

	formatTasks("remaining", result.Remaining)
	formatTasks("in_progress", result.InProgress)
	formatTasks("completed", result.Completed)
	formatTasks("blocked", result.Blocked)
	formatTasks("cancelled", result.Cancelled)

	if len(lines) == 0 {
		return mcp.NewToolResultText("No tasks"), nil
	}

	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// handleTaskNext returns the next highest priority unblocked task.
func (s *Server) handleTaskNext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Call TaskNext
	task, err := s.store.TaskNext(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	if task == nil {
		return mcp.NewToolResultText("No ready tasks"), nil
	}

	// Output JSON for parsing (matching CLI format)
	output, err := json.Marshal(map[string]any{
		"id":       task.ID,
		"content":  task.Content,
		"priority": task.Priority,
		"status":   task.Status,
	})
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: failed to marshal task: %v", err)), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

// handleNoteAdd adds one or more notes to the session.
func (s *Server) handleNoteAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultText("error: no arguments provided"), nil
	}

	// Extract notes array
	notesRaw, ok := args["notes"]
	if !ok {
		return mcp.NewToolResultText("error: missing 'notes' parameter"), nil
	}

	// Type assert to []any (mcp-go returns arrays as []any)
	notesArray, ok := notesRaw.([]any)
	if !ok {
		return mcp.NewToolResultText("error: 'notes' is not an array"), nil
	}

	if len(notesArray) == 0 {
		return mcp.NewToolResultText("error: at least one note is required"), nil
	}

	// Load state to get current iteration number
	state, err := s.store.LoadState(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: failed to load state: %v", err)), nil
	}

	// Get current iteration number (last iteration in slice)
	currentIteration := 0
	if len(state.Iterations) > 0 {
		currentIteration = state.Iterations[len(state.Iterations)-1].Number
	}

	// Add each note
	addedNotes := []string{}
	for i, noteRaw := range notesArray {
		// Convert to map[string]any
		noteMap, ok := noteRaw.(map[string]any)
		if !ok {
			return mcp.NewToolResultText(fmt.Sprintf("error: note %d is not an object", i)), nil
		}

		// Extract content (required)
		content, ok := noteMap["content"].(string)
		if !ok || content == "" {
			return mcp.NewToolResultText(fmt.Sprintf("error: note %d missing or empty 'content' field", i)), nil
		}

		// Extract type (required)
		noteType, ok := noteMap["type"].(string)
		if !ok || noteType == "" {
			return mcp.NewToolResultText(fmt.Sprintf("error: note %d missing or empty 'type' field", i)), nil
		}

		// Call NoteAdd
		note, err := s.store.NoteAdd(ctx, s.sessName, session.NoteAddParams{
			Content:   content,
			Type:      noteType,
			Iteration: currentIteration,
		})
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("error: failed to add note %d: %v", i, err)), nil
		}

		addedNotes = append(addedNotes, fmt.Sprintf("%s (%s)", note.ID, note.Type))
	}

	// Return success message with note IDs
	result := fmt.Sprintf("Added %d note(s): %s", len(addedNotes), addedNotes[0])
	for i := 1; i < len(addedNotes); i++ {
		result += fmt.Sprintf(", %s", addedNotes[i])
	}

	return mcp.NewToolResultText(result), nil
}

// handleNoteList returns all notes, optionally filtered by type.
func (s *Server) handleNoteList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments (optional for this handler)
	args := request.GetArguments()

	// Extract optional type filter
	noteType := ""
	if args != nil {
		if t, ok := args["type"].(string); ok {
			noteType = t
		}
	}

	// Call NoteList with optional filter
	notes, err := s.store.NoteList(ctx, s.sessName, session.NoteListParams{
		Type: noteType,
	})
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	// Handle empty result
	if len(notes) == 0 {
		if noteType != "" {
			return mcp.NewToolResultText(fmt.Sprintf("No notes with type '%s'", noteType)), nil
		}
		return mcp.NewToolResultText("No notes"), nil
	}

	// Format output matching CLI format: [type] (#iteration) content
	var lines []string
	for _, note := range notes {
		lines = append(lines, fmt.Sprintf("[%s] (#%d) %s", note.Type, note.Iteration, note.Content))
	}

	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// handleIterationSummary records a summary for the current iteration.
func (s *Server) handleIterationSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultText("error: no arguments provided"), nil
	}

	// Extract required summary parameter
	summary, ok := args["summary"].(string)
	if !ok || summary == "" {
		return mcp.NewToolResultText("error: missing or empty 'summary' parameter"), nil
	}

	// Load state to get current iteration number
	state, err := s.store.LoadState(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: failed to load state: %v", err)), nil
	}

	// Find the current (last) iteration number and check for existing summary
	iterNum := 1
	var currentIter *session.Iteration
	if len(state.Iterations) > 0 {
		currentIter = state.Iterations[len(state.Iterations)-1]
		iterNum = currentIter.Number
	}

	// Guard: if this iteration already has a summary, don't record a duplicate
	if currentIter != nil && currentIter.Summary != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Iteration #%d already has a summary recorded", iterNum)), nil
	}

	// Collect task IDs that are in_progress or were recently worked on
	var tasksWorked []string
	for id, task := range state.Tasks {
		if task.Status == "in_progress" || task.Iteration == iterNum {
			tasksWorked = append(tasksWorked, id)
		}
	}

	// Call IterationSummary
	err = s.store.IterationSummary(ctx, s.sessName, iterNum, summary, tasksWorked)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Summary recorded for iteration #%d", iterNum)), nil
}

// handleSessionComplete marks the session as complete.
func (s *Server) handleSessionComplete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Call SessionComplete (no parameters needed)
	err := s.store.SessionComplete(ctx, s.sessName)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("error: %v", err)), nil
	}

	return mcp.NewToolResultText("Session marked complete"), nil
}
