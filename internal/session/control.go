package session

import (
	"context"
	"fmt"

	"github.com/mark3labs/iteratr/internal/nats"
)

// SessionComplete marks a session as complete.
// Creates an event of type "control" with action "session_complete".
// This signals that all tasks are done and the iteration loop should terminate.
// Returns an error if any tasks are not in a terminal state (completed, blocked, cancelled).
func (s *Store) SessionComplete(ctx context.Context, session string) error {
	// Load current state to check task statuses
	state, err := s.LoadState(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to load session state: %w", err)
	}

	// Check that all tasks are in terminal states
	var incompleteTasks []string
	for _, task := range state.Tasks {
		switch task.Status {
		case "completed", "blocked", "cancelled":
			// Terminal states - OK
		default:
			// Non-terminal states (remaining, in_progress, etc.)
			incompleteTasks = append(incompleteTasks, task.ID)
		}
	}

	if len(incompleteTasks) > 0 {
		return fmt.Errorf("cannot complete session: %d task(s) not in terminal state (completed/blocked/cancelled). Complete all tasks before marking session complete", len(incompleteTasks))
	}

	// Create event
	event := Event{
		Session: session,
		Type:    nats.EventTypeControl,
		Action:  "session_complete",
		Data:    "Session marked as complete",
	}

	// Publish event
	_, err = s.PublishEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to publish session complete event: %w", err)
	}

	return nil
}

// SessionRestart marks a completed session as not complete, allowing it to continue.
// Creates an event of type "control" with action "session_restart".
func (s *Store) SessionRestart(ctx context.Context, session string) error {
	// Create event
	event := Event{
		Session: session,
		Type:    nats.EventTypeControl,
		Action:  "session_restart",
		Data:    "Session restarted",
	}

	// Publish event
	_, err := s.PublishEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to publish session restart event: %w", err)
	}

	return nil
}
