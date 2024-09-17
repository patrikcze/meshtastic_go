package pubsub

import (
	"strings"
	"sync"
)

type PubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan interface{}
}

func NewPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[string][]chan interface{}),
	}
}

// Subscribe to a specific topic. Returns a channel on which messages will be received.
func (ps *PubSub) Subscribe(topic string) <-chan interface{} {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan interface{}, 1)
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)
	return ch
}

// Unsubscribe from a specific topic to stop receiving messages on the channel.
func (ps *PubSub) Unsubscribe(topic string, ch <-chan interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if subs, found := ps.subscribers[topic]; found {
		for i, sub := range subs {
			if sub == ch {
				ps.subscribers[topic] = append(subs[:i], subs[i+1:]...)
				close(sub)
				break
			}
		}
	}
}

// Publish a message to a specific topic.
func (ps *PubSub) Publish(topic string, msg interface{}) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Direct topic match
	if subs, found := ps.subscribers[topic]; found {
		for _, ch := range subs {
			ch <- msg
		}
	}

	// Handle wildcard subscriptions
	for t, subs := range ps.subscribers {
		if ps.matchWildcardTopic(t, topic) {
			for _, ch := range subs {
				ch <- msg
			}
		}
	}
}

// Close closes all channels and clears the subscribers map.
func (ps *PubSub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, subs := range ps.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	ps.subscribers = nil
}

// matchWildcardTopic checks if a wildcard topic matches the given topic.
func (ps *PubSub) matchWildcardTopic(wildcard, topic string) bool {
	// Simple case where wildcard is "*" to match any topic
	if wildcard == "*" {
		return true
	}

	// Support for more complex patterns like "meshtastic.receive.*"
	wildcardParts := strings.Split(wildcard, ".")
	topicParts := strings.Split(topic, ".")

	if len(wildcardParts) != len(topicParts) {
		return false
	}

	for i := 0; i < len(wildcardParts); i++ {
		if wildcardParts[i] != "*" && wildcardParts[i] != topicParts[i] {
			return false
		}
	}
	return true
}
