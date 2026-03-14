// Package store provides data storage mechanisms for the UI components.
package store

import (
	"sync"
	"time"

	"meshtastic_go/pkg/generated"
)

// MessageType represents the type of message
type MessageType int

const (
	// MessageTypeIncoming represents a message received from another node
	MessageTypeIncoming MessageType = iota
	// MessageTypeOutgoing represents a message sent by the local node
	MessageTypeOutgoing
	// MessageTypeSystem represents a system message (not from a user)
	MessageTypeSystem
)

// MessageStatus represents the delivery status of a message
type MessageStatus int

const (
	// MessageStatusSending indicates the message is being sent
	MessageStatusSending MessageStatus = iota
	// MessageStatusSent indicates the message was sent successfully
	MessageStatusSent
	// MessageStatusDelivered indicates the message was delivered to the recipient
	MessageStatusDelivered
	// MessageStatusFailed indicates the message failed to send
	MessageStatusFailed
)

// Message represents a message with all its metadata.
type Message struct {
	ID          string        // Unique ID for the message
	Content     string        // Text content of the message
	SenderID    uint32        // NodeID of the sender
	SenderName  string        // Human-readable name of the sender
	RecipientID uint32        // NodeID of the recipient, 0xFFFFFFFF for broadcast
	ChannelID   int           // Channel ID the message was sent on
	Timestamp   time.Time     // When the message was sent/received
	Type        MessageType   // Type of message
	Status      MessageStatus // Delivery status of the message
	IsLocal     bool          // Whether this message was sent by the local node
}

// MessageStore provides thread-safe storage and retrieval of messages.
type MessageStore struct {
	messages []Message
	mu       sync.RWMutex
}

// NewMessageStore creates a new instance of MessageStore.
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make([]Message, 0, 100), // Pre-allocate for 100 messages
	}
}

// AddMessage adds a new message to the store.
func (ms *MessageStore) AddMessage(msg Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = append(ms.messages, msg)
}

// AddFromMeshPacket creates a Message from a MeshPacket and adds it to the store.
func (ms *MessageStore) AddFromMeshPacket(packet *generated.MeshPacket, nodeInfo map[uint32]string) {
	// Extract text data from the packet payload
	var content string
	var channelID int
	
	if packet.GetPayloadVariant() != nil {
		if dataPayload, ok := packet.GetPayloadVariant().(*generated.MeshPacket_Decoded); ok {
			content = string(dataPayload.Decoded.GetPayload())
			channelID = int(dataPayload.Decoded.GetPortnum())
		}
	}
	
	// Get sender name from node info map, or use ID if unknown
	senderName := nodeInfo[packet.From]
	if senderName == "" {
		senderName = formatNodeID(packet.From)
	}
	
	msg := Message{
		ID:          generateMessageID(),
		Content:     content,
		SenderID:    packet.From,
		SenderName:  senderName,
		RecipientID: packet.To,
		ChannelID:   channelID,
		Timestamp:   time.Now(), // Use current time as we may not have reliable packet time
		Type:        MessageTypeIncoming,
		Status:      MessageStatusDelivered,
		IsLocal:     false,
	}
	
	ms.AddMessage(msg)
}

// CreateOutgoingMessage creates a new outgoing message and adds it to the store.
func (ms *MessageStore) CreateOutgoingMessage(content string, recipientID uint32, channelID int, localNodeID uint32, localNodeName string) Message {
	msg := Message{
		ID:          generateMessageID(),
		Content:     content,
		SenderID:    localNodeID,
		SenderName:  localNodeName,
		RecipientID: recipientID,
		ChannelID:   channelID,
		Timestamp:   time.Now(),
		Type:        MessageTypeOutgoing,
		Status:      MessageStatusSending,
		IsLocal:     true,
	}
	
	ms.AddMessage(msg)
	return msg
}

// CreateSystemMessage adds a system message to the store.
func (ms *MessageStore) CreateSystemMessage(content string) {
	msg := Message{
		ID:          generateMessageID(),
		Content:     content,
		SenderID:    0,
		SenderName:  "System",
		RecipientID: 0,
		ChannelID:   0,
		Timestamp:   time.Now(),
		Type:        MessageTypeSystem,
		Status:      MessageStatusDelivered,
		IsLocal:     false,
	}
	
	ms.AddMessage(msg)
}

// GetAllMessages returns a copy of all messages in the store.
func (ms *MessageStore) GetAllMessages() []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	// Make a copy to avoid potential race conditions
	result := make([]Message, len(ms.messages))
	copy(result, ms.messages)
	
	return result
}

// GetMessagesByChannel returns all messages for a specific channel.
func (ms *MessageStore) GetMessagesByChannel(channelID int) []Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	var result []Message
	for _, msg := range ms.messages {
		if msg.ChannelID == channelID {
			result = append(result, msg)
		}
	}
	
	return result
}

// UpdateMessageStatus updates the status of a message identified by ID.
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

// generateMessageID creates a unique ID for a message.
// In a real implementation, this might use UUID or another robust ID generation method.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000") + "_" + randomString(6)
}

// randomString generates a random string of the specified length.
func randomString(length int) string {
	// Simple implementation for now
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[int(time.Now().UnixNano()>>uint(i*8))%len(chars)]
	}
	return string(result)
}

// formatNodeID converts a node ID to a string representation.
func formatNodeID(nodeID uint32) string {
	return "Node-" + string(rune(nodeID&0xFF)) + string(rune((nodeID>>8)&0xFF)) + string(rune((nodeID>>16)&0xFF))
}

