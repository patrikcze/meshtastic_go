package main

import (
	"log"
	"math/rand"
	"meshtastic_go/internal/protocol"
	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"

	"go.bug.st/serial"
)

func main() {
	dispatcher := transport.NewEventDispatcher()
	dispatcher.RegisterHandler("MeshPacketReceived", protocol.HandleMeshPacketReceived)

	devPath := "/dev/cu.usbmodem1101"
	mode := &serial.Mode{BaudRate: 115200, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit}

	streamConn, err := transport.NewSerialStreamConn(devPath, mode)
	if err != nil {
		log.Fatalf("Failed to open serial stream: %v", err)
	}
	defer streamConn.Close()

	state := &protocol.State{}

	err = protocol.SendConfigRequest(streamConn, rand.Uint32())
	if err != nil {
		log.Printf("Failed to send config request: %v", err)
	}

	// Example message send
	err = protocol.SendTextMessage(streamConn, 532783092, 1419948843, "Connected to Device over Serial from GO!", false)
	if err != nil {
		log.Fatalf("Failed to send text message: %v", err)
	}

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
