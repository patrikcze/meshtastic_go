package pubsub

import (
	"sync"
)

// PubSub struct manages the subscriptions and publishing of messages.
type PubSub struct {
	subscribers map[string][]chan interface{}
	mutex       sync.RWMutex
}

// NewPubSub initializes and returns a new PubSub instance.
func NewPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[string][]chan interface{}),
	}
}

// Subscribe to a topic. Returns a channel to receive messages on.
func (ps *PubSub) Subscribe(topic string) <-chan interface{} {
	ch := make(chan interface{}, 1)
	ps.mutex.Lock()
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)
	ps.mutex.Unlock()
	return ch
}

// Publish a message to a topic.
func (ps *PubSub) Publish(topic string, msg interface{}) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	if subscribers, found := ps.subscribers[topic]; found {
		for _, ch := range subscribers {
			ch <- msg
		}
	}
}
