package serial

import (
	"context"
	"fmt"
	"log"
	"meshtastic_go/internal/transport"
)

// Node represents a node in the network
type Node struct {
	ID   string
	Name string
}

// Channel represents a channel in the device configuration
type Channel struct {
	Name string
}

// Device provides a simplified API for interacting with the Meshtastic client
type Device struct {
	client *transport.Client
}

// NewDevice creates a new Device instance
func NewDevice(client *transport.Client) *Device {
	return &Device{client: client}
}

// Connect initializes the connection to the Meshtastic device
func (d *Device) Connect(ctx context.Context) error {
	err := d.client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Meshtastic device: %w", err)
	}
	return nil
}

// GetNodes retrieves the list of nodes from the Meshtastic device
func (d *Device) GetNodes() ([]Node, error) {
	nodes := d.client.State.Nodes()
	if nodes == nil {
		log.Fatal("No nodes retrieved from the device")
		return []Node{}, nil // Return an empty list instead of causing a panic
	}

	result := make([]Node, len(nodes))
	for i, n := range nodes {
		result[i] = Node{
			ID:   n.User.Id,
			Name: n.User.LongName,
		}
	}
	return result, nil
}

func (d *Device) GetChannels() ([]Channel, error) {
	channels := d.client.State.Channels()
	result := make([]Channel, len(channels))
	for i, c := range channels {
		result[i] = Channel{
			Name: c.Settings.Name,
		}
	}
	return result, nil
}
