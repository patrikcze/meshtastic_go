package transport

import (
	"time"

	"meshtastic_go/pkg/generated"
)

// RadioEvent is a marker interface for all events emitted by the Client.
// Consumers read these from Client.Events and type-switch to handle each variant.
type RadioEvent interface {
	radioEvent() // unexported marker — prevents external implementations
}

// TextMessageEvent is emitted when a decoded text message arrives from the mesh.
type TextMessageEvent struct {
	From       uint32
	To         uint32
	Channel    uint32
	Content    string
	SenderName string
	Timestamp  time.Time
	MessageID  uint32
}

func (TextMessageEvent) radioEvent() {}

// NodeUpdateEvent is emitted when a NodeInfo packet is received (initial config or later update).
type NodeUpdateEvent struct {
	Node *generated.NodeInfo
}

func (NodeUpdateEvent) radioEvent() {}

// PositionEvent is emitted when a position update is received from a node.
type PositionEvent struct {
	From     uint32
	Position *generated.Position
}

func (PositionEvent) radioEvent() {}

// TelemetryEvent is emitted when device metrics are received from a node.
type TelemetryEvent struct {
	From    uint32
	Metrics *generated.DeviceMetrics
}

func (TelemetryEvent) radioEvent() {}

// ConnectionEvent signals a change in the connection lifecycle.
type ConnectionEvent struct {
	Status  ConnectionStatus
	Message string
}

func (ConnectionEvent) radioEvent() {}

// ConnectionStatus represents the state of the device connection.
type ConnectionStatus int

const (
	StatusConnecting ConnectionStatus = iota
	StatusConnected
	StatusDisconnected
	StatusError
)

// String returns a human-readable label for the status.
func (s ConnectionStatus) String() string {
	switch s {
	case StatusConnecting:
		return "Connecting"
	case StatusConnected:
		return "Connected"
	case StatusDisconnected:
		return "Disconnected"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}
