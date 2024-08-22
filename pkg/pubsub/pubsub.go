package pubsub

import (
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

func (ps *PubSub) Subscribe(topic string) <-chan interface{} {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ch := make(chan interface{}, 1)
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)
	return ch
}

func (ps *PubSub) Publish(topic string, msg interface{}) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, ch := range ps.subscribers[topic] {
		ch <- msg
	}
}

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
