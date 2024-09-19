package transport

import (
	"sync"
)

const (
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
