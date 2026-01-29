package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/iteratr/internal/session"
	"github.com/mark3labs/mcp-go/mcp"
)

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
	// TODO: implement
	return mcp.NewToolResultText("not implemented"), nil
}

// handleNoteList returns all notes, optionally filtered by type.
func (s *Server) handleNoteList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// TODO: implement
	return mcp.NewToolResultText("not implemented"), nil
}

// handleIterationSummary records a summary for the current iteration.
func (s *Server) handleIterationSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// TODO: implement
	return mcp.NewToolResultText("not implemented"), nil
}

// handleSessionComplete marks the session as complete.
func (s *Server) handleSessionComplete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// TODO: implement
	return mcp.NewToolResultText("not implemented"), nil
}
