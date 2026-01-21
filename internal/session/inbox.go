package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/iteratr/internal/nats"
	"github.com/rs/xid"
)

// InboxAddParams represents the parameters for adding an inbox message.
type InboxAddParams struct {
	Content string `json:"content"`
}

// InboxMarkReadParams represents the parameters for marking a message as read.
type InboxMarkReadParams struct {
	ID string `json:"id"` // Message ID or prefix (8+ chars)
}

// InboxAdd creates a new inbox message in the session.
func (s *Store) InboxAdd(ctx context.Context, session string, params InboxAddParams) (*Message, error) {
	// Validate required fields
	if params.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Generate unique ID and timestamp
	id := xid.New().String()
	now := time.Now()

	// Create and publish event
	event := Event{
		ID:        id,
		Timestamp: now,
		Session:   session,
		Type:      nats.EventTypeInbox,
		Action:    "add",
		Data:      params.Content,
	}

	_, err := s.PublishEvent(ctx, event)
	if err != nil {
		return nil, err
	}

	// Build message object to return
	msg := &Message{
		ID:        id,
		Content:   params.Content,
		Read:      false,
		CreatedAt: now,
	}

	return msg, nil
}

// InboxMarkRead marks an inbox message as read.
// The ID parameter supports prefix matching (minimum 8 characters).
func (s *Store) InboxMarkRead(ctx context.Context, session string, params InboxMarkReadParams) error {
	// Validate required fields
	if params.ID == "" {
		return fmt.Errorf("message ID is required")
	}

	// Load current state to resolve message ID prefix
	state, err := s.LoadState(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Resolve message ID (supports prefix matching)
	messageID, err := resolveMessageID(state, params.ID)
	if err != nil {
		return err
	}

	// Create event metadata
	meta, _ := json.Marshal(map[string]any{
		"message_id": messageID,
	})

	// Create and publish event
	event := Event{
		Session: session,
		Type:    nats.EventTypeInbox,
		Action:  "mark_read",
		Meta:    meta,
	}

	_, err = s.PublishEvent(ctx, event)
	return err
}

// InboxList returns all unread inbox messages.
func (s *Store) InboxList(ctx context.Context, session string) ([]*Message, error) {
	// Load current state
	state, err := s.LoadState(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Filter for unread messages
	var unread []*Message
	for _, msg := range state.Inbox {
		if !msg.Read {
			unread = append(unread, msg)
		}
	}

	return unread, nil
}

// resolveMessageID resolves a message ID or prefix to a full message ID.
// Supports prefix matching with minimum 8 characters.
// Returns an error if the prefix is ambiguous or not found.
func resolveMessageID(state *State, idOrPrefix string) (string, error) {
	// Check prefix length requirement
	if len(idOrPrefix) < 8 {
		return "", fmt.Errorf("message ID prefix must be at least 8 characters (got %d)", len(idOrPrefix))
	}

	// Find matching messages by prefix or exact match
	var matches []string
	for _, msg := range state.Inbox {
		if msg.ID == idOrPrefix || strings.HasPrefix(msg.ID, idOrPrefix) {
			matches = append(matches, msg.ID)
		}
	}

	// Handle results
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("message not found: %s", idOrPrefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous message ID prefix: %s (matches %d messages)", idOrPrefix, len(matches))
	}
}
