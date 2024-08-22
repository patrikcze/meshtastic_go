package main

import (
	"bytes"
	"encoding/hex"
	"log"
	"time"
	"unicode/utf8"

	meshtastic "buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/tarm/serial"
	"google.golang.org/protobuf/proto"
)

// Constants for the Meshtastic protocol
const (
	START1          = 0x94
	START2          = 0xC3
	HEADER_LEN      = 4
	MAX_PACKET_SIZE = 512
)

// PortNum represents the different port numbers in the Meshtastic protocol.
type PortNum int32

// Enum values for PortNum.
const (
	PortNum_UNKNOWN_PORTNUM  PortNum = 0
	PortNum_TEXT_MESSAGE_APP PortNum = 1
	PortNum_POSITION_APP     PortNum = 2
	PortNum_NODEINFO_APP     PortNum = 3
	// Add other port numbers as needed...
)

// SerialInterface represents the serial connection to the Meshtastic device
type SerialInterface struct {
	port *serial.Port
}

func (si *SerialInterface) ReadPacket(buffer *bytes.Buffer) ([]byte, error) {
	for {
		buf := make([]byte, 1)
		n, err := si.port.Read(buf)
		if err != nil {
			if err.Error() == "EOF" {
				// EOF can be ignored, keep reading
				continue
			}
			return nil, err
		}
		if n == 0 {
			// No data was read, likely a timeout, so continue reading
			continue
		}

		// Append byte to the buffer
		buffer.WriteByte(buf[0])

		// Check if we have at least the header
		if buffer.Len() >= HEADER_LEN {
			packetLen := int(buffer.Bytes()[2])<<8 + int(buffer.Bytes()[3])
			totalLen := HEADER_LEN + packetLen

			// If we have the full packet, return it
			if buffer.Len() >= totalLen {
				packet := buffer.Next(totalLen) // Read the complete packet
				return packet[HEADER_LEN:], nil // Return just the payload, stripping the header
			}
		}
	}
}

// NewSerialInterface initializes a new SerialInterface.
func NewSerialInterface(devPath string, baudRate int) (*SerialInterface, error) {
	c := &serial.Config{Name: devPath, Baud: baudRate, ReadTimeout: time.Millisecond * 500}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	// Flush the port and wait to ensure the device is ready
	s.Flush()
	time.Sleep(100 * time.Millisecond)

	return &SerialInterface{port: s}, nil
}

// Close closes the serial port.
func (si *SerialInterface) Close() {
	if si.port != nil {
		si.port.Flush()
		time.Sleep(100 * time.Millisecond)
		si.port.Close()
	}
}

// isLikelyText checks if the data is likely to be a UTF-8 encoded text.
func isLikelyText(data []byte) bool {
	return utf8.Valid(data)
}

// handleMessage handles incoming messages and decodes them based on their characteristics.
func handleMessage(data []byte) {
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

	// Check if the packet contains decoded information
	if packet.GetDecoded() != nil {
		decoded := packet.GetDecoded()
		portnum := decoded.GetPortnum()

		// Filter specific Portnums (e.g., TEXT_MESSAGE_APP and POSITION_APP)
		if portnum == meshtastic.PortNum_PORTNUM_TEXT_MESSAGE_APP || portnum == meshtastic.PortNum_PORTNUM_POSITION_APP {
			payload := decoded.GetPayload()
			log.Printf("Received message on Portnum %s with payload: %s", portnum.String(), string(payload))

			// Process payload further as needed
			processPayload(portnum, payload)
		} else {
			log.Printf("Received message on unfiltered Portnum %s", portnum.String())
		}
	} else if packet.GetEncrypted() != nil {
		encryptedPayload := packet.GetEncrypted()
		log.Printf("Received encrypted payload: %s", hex.EncodeToString(encryptedPayload))

		// Add your decryption logic here if necessary
		decryptedPayload, err := decryptPayload(encryptedPayload)
		if err != nil {
			log.Printf("Failed to decrypt payload: %v", err)
		} else {
			log.Printf("Decrypted payload: %s", string(decryptedPayload))
		}
	} else {
		log.Printf("Received unknown packet type")
	}
}

// processPayload handles the decoded payload based on the Portnum.
func processPayload(portnum meshtastic.PortNum, payload []byte) {
	switch portnum {
	case meshtastic.PortNum_PORTNUM_TEXT_MESSAGE:
		log.Printf("Text Message Received: %s", string(payload))
	case meshtastic.PortNum_PORTNUM_POSITION_APP:
		log.Printf("Position Message Received: %s", string(payload))
		// You could further parse position-related payloads here
	default:
		log.Printf("Unhandled Portnum %s with payload: %s", portnum.String(), string(payload))
	}
}

// decryptPayload is a placeholder function to decrypt the payload.
// Replace this with actual decryption logic if necessary.
func decryptPayload(encryptedPayload []byte) ([]byte, error) {
	// Implement decryption here
	return encryptedPayload, nil // Placeholder, return decrypted data instead
}

func main() {
	// Set your specific device path here
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
			handleMessage(packet)
		}
	}
}
