package pubsub

import (
	"sync"
)

type Subscriber chan interface{}
type Topic string

type PubSub struct {
	subscribers map[Topic][]Subscriber
	mutex       sync.RWMutex
}

func NewPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[Topic][]Subscriber),
	}
}

func (ps *PubSub) Subscribe(topic Topic) Subscriber {
	ch := make(Subscriber)
	ps.mutex.Lock()
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)
	ps.mutex.Unlock()
	return ch
}

func (ps *PubSub) Unsubscribe(topic Topic, subscriber Subscriber) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	subs := ps.subscribers[topic]
	for i, sub := range subs {
		if sub == subscriber {
			ps.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

func (ps *PubSub) Publish(topic Topic, msg interface{}) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	for _, subscriber := range ps.subscribers[topic] {
		subscriber <- msg
	}
}
