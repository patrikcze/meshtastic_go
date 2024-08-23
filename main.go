package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"meshtastic_go/pkg/generated"
	"sync"
	"time"

	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
	// Import the generated protobuf code
)

// Define constants for START1, START2, HEADER_LEN, and MAX_TO_FROM_RADIO_SIZE
const (
	START1                 = 0x94
	START2                 = 0xC3
	HEADER_LEN             = 4
	MAX_TO_FROM_RADIO_SIZE = 512
)

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

type SerialInterface struct {
	port serial.Port
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

func (si *SerialInterface) ReadPacket(buffer *bytes.Buffer) ([]byte, error) {
	syncing := true

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

		// Log each byte read from the serial port
		log.Printf("Read byte: 0x%02X", buf[0])

		// If we are syncing, we need to find the start of the packet
		if syncing {
			if buf[0] == START1 {
				buffer.Reset()
				buffer.WriteByte(buf[0])
				syncing = false
				log.Println("Found START1 byte, looking for START2...")
				continue
			} else {
				log.Println("Unexpected byte, still looking for START1...")
				continue
			}
		}

		// Append byte to the buffer
		buffer.WriteByte(buf[0])

		// Once START1 is found, ensure START2 is next
		if buffer.Len() == 2 {
			if buffer.Bytes()[1] != START2 {
				log.Println("START2 byte not found, resynchronizing...")
				syncing = true
				buffer.Reset()
				continue
			} else {
				log.Println("START2 byte found, reading packet length...")
			}
		}

		// If we have at least the header, read the packet length
		if buffer.Len() == HEADER_LEN {
			packetLen := int(buffer.Bytes()[2])<<8 + int(buffer.Bytes()[3])
			log.Printf("Header detected: packetLen=%d", packetLen)

			if packetLen > MAX_TO_FROM_RADIO_SIZE {
				log.Printf("Invalid packet length: %d. Discarding packet.", packetLen)
				syncing = true
				buffer.Reset()
				continue
			}

			totalLen := HEADER_LEN + packetLen

			// If we have the full packet, return it
			if buffer.Len() >= totalLen {
				packet := buffer.Next(totalLen) // Read the complete packet
				log.Printf("Full packet received: %s", hex.EncodeToString(packet))

				return packet[HEADER_LEN:], nil // Return just the payload, stripping the header
			}
		}
	}
}

func handleMessage(data []byte, dispatcher *EventDispatcher) {
	log.Printf("Attempting to parse packet: %s", hex.EncodeToString(data))

	var packet generated.MeshPacket // Use the generated protobuf struct
	if err := proto.Unmarshal(data, &packet); err != nil {
		log.Printf("Failed to parse received data: %v", err)
		log.Printf("Data: %s", hex.EncodeToString(data))
		return
	}

	// Log the parsed packet details for debugging
	log.Printf("Parsed MeshPacket: From=%d, To=%d, Id=%d, Portnum=%d", packet.From, packet.To, packet.Id, packet.GetDecoded().Portnum)

	// Create an event for the received packet
	event := Event{
		Type: EventMeshPacketReceived,
		Data: &packet,
	}

	// Dispatch the event
	dispatcher.Dispatch(event)
}

func handleMeshPacketReceived(event Event) {
	packet, ok := event.Data.(*generated.MeshPacket)
	if !ok {
		return
	}

	timestamp := time.Now().Unix()
	log.Printf("Received packet at %d from %d to %d, message ID: %d",
		timestamp, packet.From, packet.To, packet.Id)

	// Check if the packet contains decoded data
	decoded := packet.GetDecoded()
	if decoded != nil {
		switch decoded.Portnum {
		case generated.PortNum_TELEMETRY_APP:
			// Decode telemetry data
			var telemetry generated.Telemetry
			if err := proto.Unmarshal(decoded.Payload, &telemetry); err != nil {
				log.Printf("Failed to parse telemetry data: %v", err)
			} else {
				// Print telemetry data in a human-readable format
				log.Printf("Telemetry received: Battery level: %.2f%%, Voltage: %.2fV, Channel Utilization: %.2f%%, Air Utilization TX: %.2f%%, Uptime: %ds",
					float64(telemetry.GetDeviceMetrics().GetBatteryLevel()),
					float64(telemetry.GetDeviceMetrics().GetVoltage()),
					float64(telemetry.GetDeviceMetrics().GetChannelUtilization()),
					float64(telemetry.GetDeviceMetrics().GetAirUtilTx()),
					telemetry.GetDeviceMetrics().GetUptimeSeconds(),
				)
			}
		case generated.PortNum_TEXT_MESSAGE_APP:
			// Decode and print text message
			log.Printf("Text message received: %s", string(decoded.Payload))
		default:
			log.Printf("Received packet with unhandled portnum: %d", decoded.Portnum)
		}
	} else {
		log.Printf("Received packet with no decoded data.")
	}
}

func main() {
	// Initialize the event dispatcher
	dispatcher := NewEventDispatcher()

	// Register event handlers
	dispatcher.RegisterHandler(EventMeshPacketReceived, handleMeshPacketReceived)

	// Serial interface setup
	devPath := "/dev/cu.usbmodem1101" // Ensure this path is correct for your device
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
		} else {
			log.Printf("No packet received.")
		}
	}
}
