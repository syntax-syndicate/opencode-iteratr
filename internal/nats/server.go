package nats

import (
	"errors"
	"time"

	"github.com/mark3labs/iteratr/internal/logger"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// StartEmbeddedNATS starts an embedded NATS server with JetStream enabled
// using the specified data directory for file-based storage.
// Returns the server instance or an error if startup fails.
func StartEmbeddedNATS(dataDir string) (*server.Server, error) {
	logger.Debug("Starting embedded NATS server with data dir: %s", dataDir)

	opts := &server.Options{
		JetStream:  true,
		StoreDir:   dataDir,
		DontListen: true, // No network ports - in-process only
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		logger.Error("Failed to create NATS server: %v", err)
		return nil, err
	}

	// Start server in background goroutine
	logger.Debug("Starting NATS server in background")
	go ns.Start()

	// Wait for server to be ready with timeout
	logger.Debug("Waiting for NATS server to be ready...")
	if !ns.ReadyForConnections(4 * time.Second) {
		logger.Error("NATS server failed to start within 4s timeout")
		return nil, errors.New("nats server failed to start within timeout")
	}

	logger.Debug("NATS server ready for connections")
	return ns, nil
}

// ConnectInProcess creates an in-process connection to the embedded NATS server.
// This connection does not use network ports and communicates directly with the server.
func ConnectInProcess(ns *server.Server) (*nats.Conn, error) {
	logger.Debug("Connecting to NATS server in-process")
	conn, err := nats.Connect("", nats.InProcessServer(ns))
	if err != nil {
		logger.Error("Failed to connect to NATS in-process: %v", err)
		return nil, err
	}
	logger.Debug("Connected to NATS successfully")
	return conn, nil
}

// CreateJetStream creates a JetStream context from a NATS connection.
// This context is used for all JetStream operations including creating streams,
// consumers, and publishing/subscribing to subjects.
func CreateJetStream(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc)
}

// Shutdown gracefully shuts down the NATS connection and server.
// It first drains and closes the connection, then shuts down the server
// with a timeout to allow in-flight operations to complete.
func Shutdown(nc *nats.Conn, ns *server.Server) error {
	logger.Debug("Starting NATS shutdown")

	// Close the connection first (drain buffered messages)
	if nc != nil {
		logger.Debug("Draining NATS connection")
		// Drain waits for published messages to be acknowledged
		// and subscriptions to complete before closing
		// Use a timeout for drain to prevent hanging
		drainDone := make(chan error, 1)
		go func() {
			drainDone <- nc.Drain()
		}()

		select {
		case err := <-drainDone:
			if err != nil {
				// Drain failed, force close
				logger.Warn("NATS drain failed, forcing close: %v", err)
				nc.Close()
			} else {
				logger.Debug("NATS connection drained successfully")
			}
		case <-time.After(2 * time.Second):
			// Drain timed out, force close
			logger.Warn("NATS drain timed out after 2s, forcing close")
			nc.Close()
		}
	}

	// Shutdown the server with a grace period
	if ns != nil {
		logger.Debug("Shutting down NATS server")
		ns.Shutdown()

		// WaitForShutdown with timeout to prevent hanging
		shutdownDone := make(chan struct{})
		go func() {
			ns.WaitForShutdown()
			close(shutdownDone)
		}()

		select {
		case <-shutdownDone:
			// Server shut down cleanly
			logger.Debug("NATS server shut down cleanly")
		case <-time.After(5 * time.Second):
			// Shutdown timed out - force stop
			// Note: There's no force-stop API, but at least we don't hang forever
			logger.Error("NATS server shutdown timed out after 5s")
			return errors.New("NATS server shutdown timed out")
		}
	}

	logger.Debug("NATS shutdown complete")
	return nil
}
