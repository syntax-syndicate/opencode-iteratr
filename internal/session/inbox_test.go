package session

import (
	"context"
	"testing"

	"github.com/mark3labs/iteratr/internal/nats"
)

func TestInboxOperations(t *testing.T) {
	// Setup: Create embedded NATS and store
	ctx := context.Background()
	ns, err := nats.StartEmbeddedNATS(t.TempDir())
	if err != nil {
		t.Fatalf("failed to start NATS: %v", err)
	}
	defer ns.Shutdown()

	nc, err := nats.ConnectInProcess(ns)
	if err != nil {
		t.Fatalf("failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nats.CreateJetStream(nc)
	if err != nil {
		t.Fatalf("failed to create JetStream: %v", err)
	}

	stream, err := nats.SetupStream(ctx, js)
	if err != nil {
		t.Fatalf("failed to setup stream: %v", err)
	}

	store := NewStore(js, stream)
	session := "test-session"

	t.Run("InboxAdd creates unread message", func(t *testing.T) {
		msg, err := store.InboxAdd(ctx, session, InboxAddParams{
			Content: "Hello from human",
		})
		if err != nil {
			t.Fatalf("InboxAdd failed: %v", err)
		}

		if msg.ID == "" {
			t.Error("expected message ID to be set")
		}
		if msg.Content != "Hello from human" {
			t.Errorf("expected content 'Hello from human', got '%s'", msg.Content)
		}
		if msg.Read {
			t.Error("expected message to be unread")
		}
		if msg.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("InboxAdd requires content", func(t *testing.T) {
		_, err := store.InboxAdd(ctx, session, InboxAddParams{
			Content: "",
		})
		if err == nil {
			t.Error("expected error for empty content")
		}
	})

	t.Run("InboxList returns unread messages", func(t *testing.T) {
		// Add multiple messages
		msg1, _ := store.InboxAdd(ctx, session, InboxAddParams{
			Content: "Message 1",
		})
		msg2, _ := store.InboxAdd(ctx, session, InboxAddParams{
			Content: "Message 2",
		})
		store.InboxAdd(ctx, session, InboxAddParams{
			Content: "Message 3",
		})

		// Mark one as read
		store.InboxMarkRead(ctx, session, InboxMarkReadParams{
			ID: msg1.ID,
		})

		// List unread messages
		unread, err := store.InboxList(ctx, session)
		if err != nil {
			t.Fatalf("InboxList failed: %v", err)
		}

		// Should have at least 2 unread (msg2 and msg3)
		if len(unread) < 2 {
			t.Errorf("expected at least 2 unread messages, got %d", len(unread))
		}

		// Verify msg1 is not in the list
		for _, msg := range unread {
			if msg.ID == msg1.ID {
				t.Error("expected msg1 to be marked as read and not in unread list")
			}
		}

		// Verify msg2 is in the list
		found := false
		for _, msg := range unread {
			if msg.ID == msg2.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected msg2 to be in unread list")
		}
	})

	t.Run("InboxMarkRead marks message as read", func(t *testing.T) {
		// Add a message
		msg, err := store.InboxAdd(ctx, session, InboxAddParams{
			Content: "Message to mark read",
		})
		if err != nil {
			t.Fatalf("InboxAdd failed: %v", err)
		}

		// Mark it as read
		err = store.InboxMarkRead(ctx, session, InboxMarkReadParams{
			ID: msg.ID,
		})
		if err != nil {
			t.Fatalf("InboxMarkRead failed: %v", err)
		}

		// Verify it's marked as read
		state, err := store.LoadState(ctx, session)
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		found := false
		for _, m := range state.Inbox {
			if m.ID == msg.ID {
				found = true
				if !m.Read {
					t.Error("expected message to be marked as read")
				}
			}
		}
		if !found {
			t.Error("message not found in state")
		}
	})

	t.Run("InboxMarkRead supports prefix matching", func(t *testing.T) {
		// Use a dedicated session to avoid conflicts with other tests
		prefixSession := "test-session-prefix"

		// Add a message
		msg, err := store.InboxAdd(ctx, prefixSession, InboxAddParams{
			Content: "Message for prefix test",
		})
		if err != nil {
			t.Fatalf("InboxAdd failed: %v", err)
		}

		// Use prefix (at least 8 chars)
		if len(msg.ID) < 8 {
			t.Skip("message ID too short for prefix test")
		}
		prefix := msg.ID[:8]

		// Mark as read using prefix
		err = store.InboxMarkRead(ctx, prefixSession, InboxMarkReadParams{
			ID: prefix,
		})
		if err != nil {
			t.Fatalf("InboxMarkRead with prefix failed: %v", err)
		}

		// Verify it's marked as read
		state, err := store.LoadState(ctx, prefixSession)
		if err != nil {
			t.Fatalf("LoadState failed: %v", err)
		}

		for _, m := range state.Inbox {
			if m.ID == msg.ID && !m.Read {
				t.Error("expected message to be marked as read")
			}
		}
	})

	t.Run("InboxMarkRead rejects short prefix", func(t *testing.T) {
		err := store.InboxMarkRead(ctx, session, InboxMarkReadParams{
			ID: "1234567", // Only 7 chars
		})
		if err == nil {
			t.Error("expected error for short prefix")
		}
	})

	t.Run("InboxMarkRead rejects non-existent ID", func(t *testing.T) {
		err := store.InboxMarkRead(ctx, session, InboxMarkReadParams{
			ID: "99999999",
		})
		if err == nil {
			t.Error("expected error for non-existent message")
		}
	})
}
