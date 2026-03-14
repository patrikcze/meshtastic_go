// Description: Main entry point for the Meshtastic Go TUI application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"

	"meshtastic_go/internal/protocol"
	"meshtastic_go/internal/transport"
	"meshtastic_go/internal/ui"
	"meshtastic_go/pkg/generated"
	"meshtastic_go/pkg/serial"
)

func main() {
	// 1. Setup logging
	log.SetLevel(log.DebugLevel)
	log.SetReportTimestamp(true)
	log.SetReportCaller(false)

	log.Info("Starting Meshtastic Go TUI application")

	// 2. Search for devices
	log.Info("Searching for Meshtastic devices...")
	ports := serial.GetPorts()
	if len(ports) == 0 {
		log.Fatalf("No suitable USB serial ports found!")
	}
	log.Info("Detected Meshtastic devices", "count", len(ports), "ports", strings.Join(ports, ", "))

	// Pick the first detected port for simplicity
	devPath := ports[0]
	log.Printf("Using serial port: %s", devPath)

	// 3. Establish a connection using the Connect function
	streamPort, err := serial.Connect(devPath)
	if err != nil {
		log.Fatalf("Failed to open serial connection: %v", err)
	}
	log.Info("Serial port opened successfully")

	// 4. Create the StreamConn object for further protocol handling
	streamConn := transport.NewRadioStreamConn(streamPort)
	if streamConn == nil {
		log.Fatalf("Failed to initialize stream connection")
	}
	log.Info("Stream connection initialized successfully")

	// Initialize the Meshtastic client
	client := transport.NewClient(streamConn, false)
	if client == nil {
		log.Fatalf("Failed to initialize Meshtastic client")
	}
	device := serial.NewDevice(client)

	// Initialize the event dispatcher
	dispatcher := transport.NewEventDispatcher()
	dispatcher.RegisterHandler("MeshPacketReceived", protocol.HandleMeshPacketReceived)

	// 5. Connect to the device
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := device.Connect(ctx); err != nil {
		log.Error("Failed to connect to Meshtastic device", "error", err)
		os.Exit(1)
	}
	log.Info("Connected to Meshtastic device successfully")

	// 6. Fetch nodes and channels
	log.Info("Fetching nodes from the device")
	nodes, err := device.GetNodes()
	if err != nil {
		log.Error("Failed to retrieve nodes", "error", err)
		os.Exit(1)
	}
	if nodes == nil {
		log.Warn("No nodes retrieved from the device")
		nodes = []serial.Node{}
	}
	log.Info("Fetching channels from the device")
	channels, err := device.GetChannels()
	if err != nil {
		log.Error("Failed to retrieve channels", "error", err)
		os.Exit(1)
	}

	// Convert nodes and channels to string slices for the TUI
	nodeNames := make([]string, len(nodes))
	for i, node := range nodes {
		nodeNames[i] = node.Name
	}
	channelNames := make([]string, len(channels))
	for i, channel := range channels {
		channelNames[i] = channel.Name
	}

	// 7. Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// 8. Create and run the TUI
	model := ui.New()
	model.UpdateNodes(nodeNames)
	model.UpdateChannels(channelNames)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		sig := <-sigChan
		log.Info("Received signal, initiating graceful shutdown", "signal", sig)
		p.Send(tea.Quit())
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Warn("Forced exit after timeout")
			os.Exit(1)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		state := &transport.State{}
		for {
			var msg generated.FromRadio
			err := streamConn.Read(&msg)
			if err != nil {
				log.Warn("Error reading from stream", "error", err)
				if strings.Contains(err.Error(), "Port has been closed") {
					return
				}
				continue
			}
			protocol.HandleMessageProto(&msg, dispatcher, state)
		}
	}()

	log.Info("Starting TUI")
	exitMsg, err := p.Run()
	if err != nil {
		log.Error("Error running TUI", "error", err)
		os.Exit(1)
	}
	if exitMsg != nil {
		log.Info("Program exited", "message", fmt.Sprintf("%v", exitMsg))
	} else {
		log.Info("Program exited")
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Allow time for cleanup
	log.Info("Performing cleanup...")
	time.Sleep(500 * time.Millisecond)
	log.Info("Meshtastic Go exited cleanly")
}
