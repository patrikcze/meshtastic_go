// Package store provides data storage mechanisms for the UI components.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MessageType represents the type of message.
type MessageType int

const (
	MessageTypeIncoming MessageType = iota
	MessageTypeOutgoing
	MessageTypeSystem
)

// MessageStatus represents the delivery status of a message.
type MessageStatus int

const (
	MessageStatusSending MessageStatus = iota
	MessageStatusSent
	MessageStatusDelivered
	MessageStatusFailed
)

// BroadcastAddr is the Meshtastic broadcast destination.
const BroadcastAddr uint32 = 0xFFFFFFFF

// Message represents a message with all its metadata.
type Message struct {
	ID          string
	Content     string
	SenderID    uint32
	SenderName  string
	RecipientID uint32 // BroadcastAddr for channel broadcasts
	ChannelIdx  uint32 // Meshtastic channel index (0 = primary)
	Timestamp   time.Time
	Type        MessageType
	Status      MessageStatus
}

// IsBroadcast returns true if the message was sent to all nodes on a channel.
func (m Message) IsBroadcast() bool {
	return m.RecipientID == BroadcastAddr
}

// MessageStore provides thread-safe storage and retrieval of messages.
type MessageStore struct {
	messages []Message
	mu       sync.RWMutex
}

// NewMessageStore creates a new instance of MessageStore.
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make([]Message, 0, 256),
	}
}

// AddMessage adds a new message to the store.
func (ms *MessageStore) AddMessage(msg Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = append(ms.messages, msg)
}

// CreateOutgoingMessage builds an outgoing message record and stores it.
func (ms *MessageStore) CreateOutgoingMessage(
	content string,
	recipientID uint32,
	channelIdx uint32,
	localNodeID uint32,
	localNodeName string,
) Message {
	msg := Message{
		ID:          generateMessageID(),
		Content:     content,
		SenderID:    localNodeID,
		SenderName:  localNodeName,
		RecipientID: recipientID,
		ChannelIdx:  channelIdx,
		Timestamp:   time.Now(),
		Type:        MessageTypeOutgoing,
		Status:      MessageStatusSending,
	}
	ms.AddMessage(msg)
	return msg
}

// CreateIncomingMessage builds an incoming message record and stores it.
func (ms *MessageStore) CreateIncomingMessage(
	content string,
	senderID uint32,
	senderName string,
	recipientID uint32,
	channelIdx uint32,
	timestamp time.Time,
) Message {
	msg := Message{
		ID:          generateMessageID(),
		Content:     content,
		SenderID:    senderID,
		SenderName:  senderName,
		RecipientID: recipientID,
		ChannelIdx:  channelIdx,
		Timestamp:   timestamp,
		Type:        MessageTypeIncoming,
		Status:      MessageStatusDelivered,
	}
	ms.AddMessage(msg)
	return msg
}

// CreateSystemMessage adds a system message to the store.
func (ms *MessageStore) CreateSystemMessage(content string) {
	ms.AddMessage(Message{
		ID:         generateMessageID(),
		Content:    content,
		SenderName: "System",
		Timestamp:  time.Now(),
		Type:       MessageTypeSystem,
		Status:     MessageStatusDelivered,
	})
}

// GetAllMessages returns a snapshot of all messages.
func (ms *MessageStore) GetAllMessages() []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	out := make([]Message, len(ms.messages))
	copy(out, ms.messages)
	return out
}

// GetMessagesByChannel returns messages matching the given channel index,
// plus all system messages (so the user always sees status updates).
func (ms *MessageStore) GetMessagesByChannel(channelIdx uint32) []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var out []Message
	for _, m := range ms.messages {
		if m.Type == MessageTypeSystem || m.ChannelIdx == channelIdx {
			out = append(out, m)
		}
	}
	return out
}

// GetMessagesByConversation returns DM messages between localNodeID and remoteNodeID,
// plus all system messages.
func (ms *MessageStore) GetMessagesByConversation(localNodeID, remoteNodeID uint32) []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var out []Message
	for _, m := range ms.messages {
		if m.Type == MessageTypeSystem {
			out = append(out, m)
			continue
		}
		// DM from remote to us, or from us to remote (non-broadcast)
		if m.IsBroadcast() {
			continue
		}
		sentByRemote := m.SenderID == remoteNodeID && m.RecipientID == localNodeID
		sentByLocal := m.SenderID == localNodeID && m.RecipientID == remoteNodeID
		if sentByRemote || sentByLocal {
			out = append(out, m)
		}
	}
	return out
}

// UpdateMessageStatus updates the status of a message by ID.
func (ms *MessageStore) UpdateMessageStatus(id string, status MessageStatus) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for i := range ms.messages {
		if ms.messages[i].ID == id {
			ms.messages[i].Status = status
			return true
		}
	}
	return false
}

// ---------- helpers ----------

// generateMessageID creates a unique ID using a timestamp plus random bytes.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000") + "_" + randomHex(4)
}

// randomHex returns a hex-encoded string of n random bytes (2*n chars).
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: extremely unlikely, but don't panic.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// FormatNodeID converts a node ID to the standard Meshtastic hex format.
func FormatNodeID(nodeID uint32) string {
	return fmt.Sprintf("!%08x", nodeID)
}
