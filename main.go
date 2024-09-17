package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"meshtastic_go/pkg/generated"
	"sync"
	"time"

	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

const (
	START1                  = 0x94
	START2                  = 0xC3
	HEADER_LEN              = 4
	MAX_TO_FROM_RADIO_SIZE  = 512
	EventMeshPacketReceived = "MeshPacketReceived"
)

type EventType string
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

type StreamConn struct {
	conn serial.Port
	mu   sync.Mutex
}

// Write method for StreamConn, now using writeStreamHeader
func (sc *StreamConn) Write(in proto.Message) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Marshal the protobuf message to bytes
	data, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("error marshalling proto message: %w", err)
	}

	// Ensure message is not too large
	if len(data) > MAX_TO_FROM_RADIO_SIZE {
		return fmt.Errorf("data length exceeds MTU: %d > %d", len(data), MAX_TO_FROM_RADIO_SIZE)
	}

	// Use the new writeStreamHeader function to write the message header
	err = writeStreamHeader(sc.conn, uint16(len(data)))
	if err != nil {
		return fmt.Errorf("error writing stream header: %w", err)
	}

	// Write the actual protobuf message
	_, err = sc.conn.Write(data)
	if err != nil {
		return fmt.Errorf("error writing proto message: %w", err)
	}

	return nil
}

func writeStreamHeader(w io.Writer, dataLen uint16) error {
	header := bytes.NewBuffer(nil)
	header.WriteByte(START1)
	header.WriteByte(START2)
	err := binary.Write(header, binary.BigEndian, dataLen)
	if err != nil {
		return fmt.Errorf("writing length to buffer: %w", err)
	}
	_, err = w.Write(header.Bytes())
	return err
}

// Close closes the underlying serial connection.
func (sc *StreamConn) Close() error {
	return sc.conn.Close()
}

func NewRadioStreamConn(port serial.Port) *StreamConn {
	return &StreamConn{conn: port}
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

type State struct {
	nodeInfo       *generated.MyNodeInfo
	deviceMetadata *generated.DeviceMetadata
	nodes          []*generated.NodeInfo
	channels       []*generated.Channel
	configs        []*generated.Config
	modules        []*generated.ModuleConfig
}

func (s *State) SetNodeInfo(nodeInfo *generated.MyNodeInfo) {
	s.nodeInfo = nodeInfo
}

func (s *State) SetDeviceMetadata(deviceMetadata *generated.DeviceMetadata) {
	s.deviceMetadata = deviceMetadata
}

func (s *State) AddNode(node *generated.NodeInfo) {
	s.nodes = append(s.nodes, node)
}

func (s *State) AddChannel(channel *generated.Channel) {
	s.channels = append(s.channels, channel)
}

func (s *State) AddConfig(config *generated.Config) {
	s.configs = append(s.configs, config)
}

func (s *State) AddModule(module *generated.ModuleConfig) {
	s.modules = append(s.modules, module)
}

func handleMessageProto(msg *generated.FromRadio, dispatcher *EventDispatcher, state *State) {
	log.Printf("Raw message received: %+v", msg)

	switch msg.GetPayloadVariant().(type) {
	case *generated.FromRadio_MyInfo:
		state.SetNodeInfo(msg.GetMyInfo())
		log.Printf("Node info received: %+v", msg.GetMyInfo())
	case *generated.FromRadio_Metadata:
		state.SetDeviceMetadata(msg.GetMetadata())
		log.Printf("Device metadata received: %+v", msg.GetMetadata())
	case *generated.FromRadio_NodeInfo:
		node := msg.GetNodeInfo()
		state.AddNode(node)
		log.Printf("Node info added: %+v", node)
	case *generated.FromRadio_Channel:
		channel := msg.GetChannel()
		state.AddChannel(channel)
		log.Printf("Channel info received: %+v", channel)
	case *generated.FromRadio_Config:
		cfg := msg.GetConfig()
		state.AddConfig(cfg)
		log.Printf("Config received: %+v", cfg)
	case *generated.FromRadio_ModuleConfig:
		cfg := msg.GetModuleConfig()
		state.AddModule(cfg)
		log.Printf("Module config received: %+v", cfg)
	case *generated.FromRadio_Packet:
		packet := msg.GetPacket()
		log.Printf("Packet received: %+v", packet)
		dispatcher.Dispatch(Event{Type: EventMeshPacketReceived, Data: packet})
	case *generated.FromRadio_FileInfo:
		fileInfo := msg.GetFileInfo()
		log.Printf("File info received: %s (%d bytes)", fileInfo.FileName, fileInfo.SizeBytes)
	case *generated.FromRadio_MqttClientProxyMessage:
		mqttMessage := msg.GetMqttClientProxyMessage()
		log.Printf("MQTT Proxy message received: topic: %s, data: %s", mqttMessage.Topic, string(mqttMessage.GetData()))
		// You can dispatch the message to an event if needed or handle it here.
	default:
		log.Printf("Unknown message type received: %+v", msg)
	}
}

func handleMeshPacketReceived(event Event) {
	packet, ok := event.Data.(*generated.MeshPacket)
	if !ok {
		log.Println("Failed to cast event data to MeshPacket")
		return
	}

	log.Printf("Received packet from %d to %d, message ID: %d", packet.From, packet.To, packet.Id)

	decoded := packet.GetDecoded()
	if decoded != nil {
		switch decoded.Portnum {
		case generated.PortNum_TEXT_MESSAGE_APP:
			log.Printf("Text message received: %s", string(decoded.Payload))
		case generated.PortNum_POSITION_APP:
			log.Printf("Position message received: %v", decoded.Payload)
		case generated.PortNum_TELEMETRY_APP:
			log.Printf("Telemetry message received: %v", decoded.Payload)
		// Add more port numbers here as needed
		default:
			// Silently skip unknown port numbers
			return
		}
	} else {
		log.Printf("Received packet with no decoded data")
	}
}

func NewSerialStreamConn(devPath string, mode *serial.Mode) (*StreamConn, error) {
	port, err := serial.Open(devPath, mode) // Pass mode instead of baudRate
	if err != nil {
		return nil, err
	}
	return NewRadioStreamConn(port), nil
}

func (sc *StreamConn) WriteWake() error {
	// Send 32 bytes of Start2 to wake the radio if sleeping.
	_, err := sc.conn.Write(bytes.Repeat([]byte{START2}, 32))
	if err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond) // Wait 100ms after sending wake
	return nil
}

func (sc *StreamConn) Read(out proto.Message) error {
	header := make([]byte, HEADER_LEN)

	for {
		// Read the header
		_, err := io.ReadFull(sc.conn, header)
		if err != nil {
			return fmt.Errorf("error reading header: %w", err)
		}

		// Check if the start bytes are correct
		if header[0] != START1 || header[1] != START2 {
			// Ignore invalid start bytes without logging
			continue
		}

		// Read the message length (third and fourth bytes)
		length := int(header[2])<<8 | int(header[3])
		if length > MAX_TO_FROM_RADIO_SIZE {
			// Ignore invalid length without logging
			continue
		}

		// Read the message body
		message := make([]byte, length)
		_, err = io.ReadFull(sc.conn, message)
		if err != nil {
			return fmt.Errorf("error reading message body: %w", err)
		}

		// Unmarshal the protobuf message
		return proto.Unmarshal(message, out)
	}
}

func main() {
	// Initialize the event dispatcher
	dispatcher := NewEventDispatcher()

	// Register event handlers
	dispatcher.RegisterHandler(EventMeshPacketReceived, handleMeshPacketReceived)

	// Serial stream setup
	devPath := "/dev/cu.usbmodem1101" // Ensure this path is correct for your device
	baudRate := 115200

	// Define serial communication mode (parameters)
	mode := &serial.Mode{
		BaudRate: baudRate,          // Baud rate for the device
		DataBits: 8,                 // Typically 8 data bits
		Parity:   serial.NoParity,   // No parity (can be OddParity or EvenParity if required)
		StopBits: serial.OneStopBit, // 1 stop bit
	}

	// Open the serial stream connection with the defined mode
	streamConn, err := NewSerialStreamConn(devPath, mode) // Pass mode instead of just baudRate
	if err != nil {
		log.Fatalf("Failed to open serial stream: %v", err)
	}
	defer streamConn.Close()

	state := &State{}

	// Send initial config request
	err = streamConn.Write(&generated.ToRadio{
		PayloadVariant: &generated.ToRadio_WantConfigId{
			WantConfigId: rand.Uint32(),
		},
	})
	if err != nil {
		log.Printf("Failed to send config request: %v", err)
	}

	for {
		// Read incoming protobuf messages
		var msg generated.FromRadio
		err := streamConn.Read(&msg)
		if err != nil {
			log.Printf("Error reading from stream: %v", err)
			continue
		}
		// Handle the received message
		handleMessageProto(&msg, dispatcher, state)
	}
}
