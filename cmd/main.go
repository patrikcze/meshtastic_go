// Description: Main entry point for the Meshtastic Go TUI application.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"

	"meshtastic_go/internal/ui"
	"meshtastic_go/pkg/serial"
)

func main() {
	// 1. Setup logging
	log.SetLevel(log.DebugLevel)
	log.SetReportTimestamp(true)
	log.SetReportCaller(false)
	// Create a file for logging
	logFile, err := os.OpenFile("meshtastic_go.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	} else {
		log.Warn("Failed to open log file, using stderr", "error", err)
	}

	log.Info("Starting Meshtastic Go TUI application")

	// 2. Search for devices
	log.Info("Searching for Meshtastic devices...")
	ports := serial.GetPorts()
	// Log detected devices
	if len(ports) > 0 {
		log.Info("Detected Meshtastic devices", "count", len(ports), "ports", strings.Join(ports, ", "))
	} else {
		log.Error("No Meshtastic devices detected")
		fmt.Println("No Meshtastic devices detected. Please connect a device and try again.")
		os.Exit(1)
	}
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Create a channel to signal program completion
	done := make(chan struct{})

	// 4. Create and run the TUI
	// Initialize our TUI model
	model := ui.New()

	// Create the bubble tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use the alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Handle signals in a separate goroutine
	go func() {
		sig := <-sigChan
		log.Info("Received signal, initiating graceful shutdown", "signal", sig)
		// Request the program to exit gracefully
		p.Send(tea.Quit())
		// If the program doesn't exit in 5 seconds, force exit
		select {
		case <-done:
			// Normal exit, do nothing
		case <-time.After(5 * time.Second):
			log.Warn("Forced exit after timeout")
			os.Exit(1)
		}
	}()

	// Run the TUI
	log.Info("Starting TUI")
	exitMsg, err := p.Run()
	if err != nil {
		log.Error("Error running program", "error", err)
		fmt.Printf("Error running Meshtastic Go: %v\n", err)
		os.Exit(1)
	}
	// Log exit message if available
	if exitMsg != nil {
		log.Info("Program exited", "message", fmt.Sprintf("%v", exitMsg))
	} else {
		log.Info("Program exited")
	}

	// 5. Handle cleanup on exit
	// Signal that we're done
	close(done)

	// Allow time for any cleanup operations (like closing connections)
	log.Info("Performing cleanup...")
	time.Sleep(500 * time.Millisecond) // Give a moment for cleanup
	log.Info("Meshtastic Go exited cleanly")
}
