package transport

import (
	"sync"
)

const (
	// EventMeshPacketReceived is the event type for when a mesh packet is received.
	EventMeshPacketReceived = "MeshPacketReceived"
)

// EventType is a string representing the type of event.
type EventType string

// Event represents an event with a type and data.
type Event struct {
	Type EventType
	Data interface{}
}

// EventHandler is a function that handles an event.
type EventHandler func(event Event)

// EventDispatcher is a dispatcher for events.
type EventDispatcher struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
}

// NewEventDispatcher creates a new event dispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[EventType][]EventHandler),
	}
}

// RegisterHandler registers an event handler for a specific event type.
func (d *EventDispatcher) RegisterHandler(eventType EventType, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Dispatch sends an event to all registered handlers for the event type.
func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if handlers, found := d.handlers[event.Type]; found {
		for _, handler := range handlers {
			go handler(event)
		}
	}
}
