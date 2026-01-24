package nats

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mark3labs/iteratr/internal/logger"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// PortFileName is the name of the file that stores the server port number
const PortFileName = "server.port"

// StartEmbeddedNATS starts an embedded NATS server with JetStream enabled
// using the specified data directory for file-based storage.
// The server listens on localhost with an available port, which is written
// to a port file for external tool processes to connect.
// Returns the server instance, port number, and an error if startup fails.
func StartEmbeddedNATS(dataDir string) (*server.Server, int, error) {
	logger.Debug("Starting embedded NATS server with data dir: %s", dataDir)

	// Find an available port
	port, err := findAvailablePort()
	if err != nil {
		logger.Error("Failed to find available port: %v", err)
		return nil, 0, fmt.Errorf("failed to find available port: %w", err)
	}
	logger.Debug("Found available port: %d", port)

	opts := &server.Options{
		JetStream: true,
		StoreDir:  dataDir,
		Host:      "127.0.0.1", // Localhost only for security
		Port:      port,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		logger.Error("Failed to create NATS server: %v", err)
		return nil, 0, err
	}

	// Start server in background goroutine
	logger.Debug("Starting NATS server in background")
	go ns.Start()

	// Wait for server to be ready with timeout
	logger.Debug("Waiting for NATS server to be ready...")
	if !ns.ReadyForConnections(4 * time.Second) {
		logger.Error("NATS server failed to start within 4s timeout")
		return nil, 0, errors.New("nats server failed to start within timeout")
	}

	// Write port to file for external processes
	portFilePath := filepath.Join(dataDir, PortFileName)
	if err := os.WriteFile(portFilePath, []byte(strconv.Itoa(port)), 0644); err != nil {
		logger.Error("Failed to write port file: %v", err)
		ns.Shutdown()
		return nil, 0, fmt.Errorf("failed to write port file: %w", err)
	}
	logger.Debug("Port file written: %s", portFilePath)

	logger.Debug("NATS server ready for connections on port %d", port)
	return ns, port, nil
}

// findAvailablePort finds an available TCP port on localhost
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// ReadPort reads the NATS port from the port file in the data directory
func ReadPort(dataDir string) (int, error) {
	portFilePath := filepath.Join(dataDir, PortFileName)
	data, err := os.ReadFile(portFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read port file: %w", err)
	}
	port, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid port in port file: %w", err)
	}
	return port, nil
}

// TryConnectExisting attempts to connect to an existing NATS server.
// Returns the connection if successful, or nil if no server is running.
// This is used to detect if another iteratr instance is already running
// and connect to it instead of starting a new server.
func TryConnectExisting(dataDir string) *nats.Conn {
	port, err := ReadPort(dataDir)
	if err != nil {
		// No port file = no server running
		return nil
	}

	// Try to connect with short timeout
	url := fmt.Sprintf("nats://127.0.0.1:%d", port)
	nc, err := nats.Connect(url, nats.Timeout(500*time.Millisecond))
	if err != nil {
		// Server not responding = stale port file
		logger.Debug("Found port file but server not responding: %v", err)
		return nil
	}

	logger.Debug("Connected to existing NATS server on port %d", port)
	return nc
}

// ConnectToPort connects to a NATS server on the specified port
func ConnectToPort(port int) (*nats.Conn, error) {
	url := fmt.Sprintf("nats://127.0.0.1:%d", port)
	logger.Debug("Connecting to NATS at %s", url)
	conn, err := nats.Connect(url)
	if err != nil {
		logger.Error("Failed to connect to NATS: %v", err)
		return nil, err
	}
	logger.Debug("Connected to NATS successfully")
	return conn, nil
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
