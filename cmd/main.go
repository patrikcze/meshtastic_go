// Description: Main entry point for the Meshtastic Go TUI application.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"

	"meshtastic_go/internal/transport"
	"meshtastic_go/internal/ui"
	"meshtastic_go/pkg/serial"
)

func main() {
	// Logging goes to a file so it doesn't interfere with the TUI alt-screen.
	logFile, err := os.OpenFile("meshtastic_go.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetLevel(log.DebugLevel)
	log.SetReportTimestamp(true)

	log.Info("Starting Meshtastic Go TUI")

	// 1. Discover devices.
	ports := serial.GetPorts()
	if len(ports) == 0 {
		fmt.Fprintln(os.Stderr, "No Meshtastic USB devices found.")
		os.Exit(1)
	}
	devPath := ports[0]
	log.Info("Using serial port", "port", devPath, "all", strings.Join(ports, ", "))

	// 2. Open serial connection.
	streamPort, err := serial.Connect(devPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}

	// 3. Wrap in StreamConn and create Client.
	streamConn := transport.NewRadioStreamConn(streamPort)
	client := transport.NewClient(streamConn)

	// 4. Config handshake (blocks until complete or timeout).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Device handshake failed: %v\n", err)
		os.Exit(1)
	}
	log.Info("Connected", "node", client.LocalNodeName())

	// 5. Build TUI and run.
	model := ui.New(client)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	// 6. Cleanup.
	if err := client.Close(); err != nil {
		log.Warn("Error closing connection", "err", err)
	}
	log.Info("Exited cleanly")
}
