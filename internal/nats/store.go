package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/iteratr/internal/logger"
	"github.com/nats-io/nats.go/jetstream"
)

// Subject pattern constants and helpers
const (
	// StreamName is the name of the JetStream stream for iteratr events
	StreamName = "iteratr_events"

	// Event types
	EventTypeTask      = "task"
	EventTypeNote      = "note"
	EventTypeInbox     = "inbox"
	EventTypeIteration = "iteration"
	EventTypeControl   = "control"
)

// SubjectForSession returns the wildcard subject pattern for all events in a session.
// Example: "iteratr.mysession.>"
func SubjectForSession(session string) string {
	return fmt.Sprintf("iteratr.%s.>", session)
}

// SubjectForEvent returns the specific subject for an event type in a session.
// Example: "iteratr.mysession.task"
func SubjectForEvent(session, eventType string) string {
	return fmt.Sprintf("iteratr.%s.%s", session, eventType)
}

// SetupStream creates or updates the JetStream stream for iteratr events.
// The stream captures all events for all sessions with 30-day retention.
// Subject pattern: iteratr.> matches all sessions and event types.
func SetupStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	logger.Debug("Setting up JetStream stream: %s", StreamName)
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     StreamName,
		Subjects: []string{"iteratr.>"}, // Match all iteratr events
		Storage:  jetstream.FileStorage,
		MaxAge:   30 * 24 * time.Hour, // 30 day retention
	})
	if err != nil {
		logger.Error("Failed to create/update stream: %v", err)
		return nil, err
	}
	logger.Debug("JetStream stream ready: %s", StreamName)
	return stream, nil
}

// CreateConsumer creates a durable consumer for reading event history.
// The consumer starts from the beginning and requires explicit acknowledgment.
func CreateConsumer(ctx context.Context, stream jetstream.Stream, name string) (jetstream.Consumer, error) {
	return stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       name,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy, // Start from beginning
	})
}

// PurgeSession removes all events for a specific session from the stream.
// This effectively resets the session to a fresh state.
func PurgeSession(ctx context.Context, stream jetstream.Stream, session string) error {
	subject := SubjectForSession(session)
	logger.Info("Purging session data for '%s' (subject: %s)", session, subject)
	return stream.Purge(ctx, jetstream.WithPurgeSubject(subject))
}
