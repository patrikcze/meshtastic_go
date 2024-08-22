package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"time"
	"unicode/utf8"

	mshproto "meshtastic_go/pkg/mshproto"
	"meshtastic_go/pkg/pubsub"

	meshtastic "buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

// SerialInterface represents the serial connection to the Meshtastic device.
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

		// Check if we have at least the header
		if buffer.Len() >= mshproto.HEADER_LEN {
			packetLen := int(buffer.Bytes()[2])<<8 + int(buffer.Bytes()[3])
			totalLen := mshproto.HEADER_LEN + packetLen

			// If we have the full packet, return it
			if buffer.Len() >= totalLen {
				packet := buffer.Next(totalLen)          // Read the complete packet
				return packet[mshproto.HEADER_LEN:], nil // Return just the payload, stripping the header
			}
		}
	}
}

// NewSerialInterface initializes a new SerialInterface.
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

// Close closes the serial port.
func (si *SerialInterface) Close() {
	if si.port != nil {
		si.port.Close()
	}
}

// isLikelyText checks if the data is likely to be a UTF-8 encoded text.
func isLikelyText(data []byte) bool {
	return utf8.Valid(data)
}

// handleMessage handles incoming messages and decodes them based on their characteristics.
func handleMessage(data []byte, ps *pubsub.PubSub) {
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

	// Publish the packet to the "meshtastic.receive" topic
	ps.Publish("meshtastic.receive", &packet)
}

// onReceive processes received MeshPackets.
func onReceive(packet *meshtastic.MeshPacket) {
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
	// Initialize PubSub system
	ps := pubsub.NewPubSub()

	// Subscribe to receive MeshPackets
	packetChannel := ps.Subscribe("meshtastic.receive")
	go func() {
		for packet := range packetChannel {
			if meshPacket, ok := packet.(*meshtastic.MeshPacket); ok {
				onReceive(meshPacket)
			}
		}
	}()

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
			handleMessage(packet, ps)
		}
	}
}
