// Description: Main entry point for the Meshtastic Go CLI application.
package main

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"meshtastic_go/internal/protocol"
	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"
	"meshtastic_go/pkg/serial"
)

func main() {
	// Step 1: Detect available USB serial ports for known devices
	ports := serial.GetPorts()
	if len(ports) == 0 {
		log.Fatalf("No suitable USB serial ports found!")
	}

	// Pick the first detected port for simplicity (can be expanded to handle multiple devices)
	devPath := ports[0]
	log.Printf("Using serial port: %s", devPath)

	// Step 2: Establish a connection using the Connect function
	streamPort, err := serial.Connect(devPath)
	if err != nil {
		log.Fatalf("Failed to open serial connection: %v", err)
	}
	defer streamPort.Close()

	// Step 3: Create the StreamConn object for further protocol handling
	streamConn := transport.NewRadioStreamConn(streamPort)

	dispatcher := transport.NewEventDispatcher()
	dispatcher.RegisterHandler("MeshPacketReceived", protocol.HandleMeshPacketReceived)
	// Register new handler for ConfigCompleteId
	//dispatcher.RegisterHandler("ConfigCompleteId", handleConfigCompletee)

	// Initialize protocol state
	state := &transport.State{}

	// Step 4: Send configuration request
	// Inside the main function
	var configID uint32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &configID); err != nil {
		log.Fatalf("failed to generate random config ID: %v", err)
	}
	err = protocol.SendConfigRequest(streamConn, configID)
	if err != nil {
		log.Printf("Failed to send config request: %v", err)
	}

	// Step 5: Send a test text message
	err = protocol.SendTextMessage(streamConn, 532783092, 1419948843, "Connected to Device over Serial from GO!", false)
	if err != nil {
		log.Fatalf("Failed to send text message: %v", err)
	}

	// Step 6: Continuously read incoming messages from the radio device
	for {
		var msg generated.FromRadio
		err := streamConn.Read(&msg)
		if err != nil {
			log.Printf("Error reading from stream: %v", err)
			continue
		}
		protocol.HandleMessageProto(&msg, dispatcher, state)
	}
}
