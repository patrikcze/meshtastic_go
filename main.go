package main

import (
	"log"
	"time" // Adjust this to your pubsub package

	// Adjust this to your pubsub package
	"pubsub"

	pb "google.golang.org/protobuf/proto" // Adjust this to your protobuf package

	"github.com/tarm/serial"
)

// Define a global channel for receiving messages
var messageChannel = make(chan *pb.Message)

type SerialInterface struct {
	port *serial.Port
}

// NewSerialInterface initializes a new SerialInterface.
func NewSerialInterface(portName string, baudRate int) (*SerialInterface, error) {
	c := &serial.Config{Name: portName, Baud: baudRate, ReadTimeout: time.Millisecond * 500}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	return &SerialInterface{port: s}, nil
}

// Close closes the serial port.
func (si *SerialInterface) Close() {
	if si.port != nil {
		si.port.Close()
	}
}

func onReceive(data []byte, si *SerialInterface) {
	// Assume `pb.Packet` is the generated protobuf struct
	log.Printf("Received data: %v", data)

}

// ReadPacket reads a packet from the serial port and returns it as a byte slice.
func (si *SerialInterface) ReadPacket() ([]byte, error) {
	buf := make([]byte, 512)
	n, err := si.port.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func sendText(si *SerialInterface, text string) {

	// Send the message

}

func main() {

	portName := "/dev/cu.usbmodem1101"
	baudRate := 115200

	si, err := NewSerialInterface(portName, baudRate)
	if err != nil {
		log.Fatalf("Failed to open serial port: %v", err)
	}
	defer si.Close()

	// Subscribe to messages
	subscriber := pubsub.Subscribe("meshtastic.receive")

	// Start listening to received messages
	go func() {
		for msg := range subscriber {
			onReceive(msg.([]byte), si)
		}
	}()

	for {
		data, err := si.ReadPacket()
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			continue
		}
		if data == nil {
			continue
		}

		// Publish received data to the subscribers
		pubsub.Publish("meshtastic.receive", data)
	}

}
