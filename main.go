package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"sync"
	"time"
	"unicode/utf8"

	mshproto "pkg/mshproto"

	meshtastic "buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

// Event Types and Handlers
type EventType string

const (
	EventMeshPacketReceived EventType = "MeshPacketReceived"
)

type Event struct {
	Type EventType
	Data interface{}
}

type EventHandler func(event Event)

type EventDispatcher struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
}

func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[EventType][]EventHandler),
	}
}

func (d *EventDispatcher) RegisterHandler(eventType EventType, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if handlers, found := d.handlers[event.Type]; found {
		for _, handler := range handlers {
			go handler(event)
		}
	}
}

// SerialInterface
type SerialInterface struct {
	port serial.Port
}

func (si *SerialInterface) ReadPacket(buffer *bytes.Buffer) ([]byte, error) {
	for {
		buf := make([]byte, 1)
		n, err := si.port.Read(buf)
		if err != nil {
			if err == io.EOF {
				continue
			}
			return nil, err
		}
		if n == 0 {
			continue
		}

		// Append byte to the buffer
		buffer.WriteByte(buf[0])

		// Check for START1 and START2 bytes at the beginning of the buffer
		if buffer.Len() >= 4 && buffer.Bytes()[0] == mshproto.START1 && buffer.Bytes()[1] == mshproto.START2 {
			packetLen := int(buffer.Bytes()[2])<<8 + int(buffer.Bytes()[3])
			totalLen := 4 + packetLen // Assuming HEADER_LEN is 4

			// If we have the full packet, return it
			if buffer.Len() >= totalLen {
				packet := buffer.Next(totalLen) // Read the complete packet
				return packet[4:], nil          // Return just the payload, stripping the header
			}
		} else {
			// If the start bytes are not found, discard the buffer
			buffer.Reset()
		}
	}
}

func NewSerialInterface(devPath string, baudRate int) (*SerialInterface, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
	}
	port, err := serial.Open(devPath, mode)
	if err != nil {
		return nil, err
	}
	return &SerialInterface{port: port}, nil
}

func (si *SerialInterface) Close() {
	if si.port != nil {
		si.port.Close()
	}
}

func isLikelyText(data []byte) bool {
	return utf8.Valid(data)
}

func handleMessage(data []byte, dispatcher *EventDispatcher) {
	if isLikelyText(data) {
		log.Printf("Received Debug Message: %s", string(data))
		return
	}

	log.Printf("Raw Data: %s", hex.EncodeToString(data))

	var packet meshtastic.MeshPacket
	if err := proto.Unmarshal(data, &packet); err != nil {
		log.Printf("Failed to parse received data: %v", err)
		return
	}

	// Create an event for the received packet
	event := Event{
		Type: EventMeshPacketReceived,
		Data: &packet,
	}

	// Dispatch the event
	dispatcher.Dispatch(event)
}

func handleMeshPacketReceived(event Event) {
	packet, ok := event.Data.(*meshtastic.MeshPacket)
	if !ok {
		return
	}

	timestamp := time.Now().Unix()
	fromNodeNumber := packet.From
	toNodeNumber := packet.To
	messageID := packet.Id
	portnum := packet.GetDecoded().Portnum
	text := packet.GetDecoded().String()
	channel := packet.Channel
	hopLimit := packet.HopLimit
	hopStart := packet.HopStart
	rxTime := packet.RxTime

	log.Printf("Received packet at %d from %d to %d, message ID: %d, portnum: %s, text: %s, channel: %d, hop limit: %d, hop start: %d, rx time: %d",
		timestamp, fromNodeNumber, toNodeNumber, messageID, portnum, text, channel, hopLimit, hopStart, rxTime)
}

func main() {
	// Initialize the event dispatcher
	dispatcher := NewEventDispatcher()

	// Register event handlers
	dispatcher.RegisterHandler(EventMeshPacketReceived, handleMeshPacketReceived)

	// Serial interface setup
	devPath := "/dev/cu.usbmodem1101"
	baudRate := 115200

	si, err := NewSerialInterface(devPath, baudRate)
	if err != nil {
		log.Fatalf("Failed to open serial port: %v", err)
	}
	defer si.Close()

	buffer := bytes.NewBuffer(nil)

	for {
		packet, err := si.ReadPacket(buffer)
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			continue
		}
		if packet != nil {
			handleMessage(packet, dispatcher)
		}
	}
}
