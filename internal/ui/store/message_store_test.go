package store

import (
	"testing"
	"time"
)

func TestAddAndGetAll(t *testing.T) {
	ms := NewMessageStore()
	ms.CreateSystemMessage("hello")
	ms.CreateSystemMessage("world")

	all := ms.GetAllMessages()
	if len(all) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(all))
	}
	if all[0].Content != "hello" || all[1].Content != "world" {
		t.Fatal("wrong content")
	}
}

func TestCreateOutgoingMessage(t *testing.T) {
	ms := NewMessageStore()
	msg := ms.CreateOutgoingMessage("hi", BroadcastAddr, 0, 42, "Me")

	if msg.Type != MessageTypeOutgoing {
		t.Fatal("expected outgoing type")
	}
	if msg.SenderID != 42 || msg.SenderName != "Me" {
		t.Fatalf("wrong sender: %d %q", msg.SenderID, msg.SenderName)
	}
	if msg.RecipientID != BroadcastAddr {
		t.Fatal("expected broadcast")
	}
	if msg.Status != MessageStatusSending {
		t.Fatal("expected sending status")
	}
	if msg.ID == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestCreateIncomingMessage(t *testing.T) {
	ms := NewMessageStore()
	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	msg := ms.CreateIncomingMessage("hello", 10, "Alice", 42, 0, ts)

	if msg.Type != MessageTypeIncoming {
		t.Fatal("expected incoming type")
	}
	if msg.SenderID != 10 || msg.RecipientID != 42 {
		t.Fatal("wrong sender/recipient")
	}
	if !msg.Timestamp.Equal(ts) {
		t.Fatal("timestamp not preserved")
	}
}

func TestGetMessagesByChannel(t *testing.T) {
	ms := NewMessageStore()
	ms.CreateSystemMessage("sys") // channel 0 / system
	ms.CreateIncomingMessage("ch0", 1, "A", BroadcastAddr, 0, time.Now())
	ms.CreateIncomingMessage("ch1", 2, "B", BroadcastAddr, 1, time.Now())
	ms.CreateIncomingMessage("ch0-2", 3, "C", BroadcastAddr, 0, time.Now())

	ch0 := ms.GetMessagesByChannel(0)
	// Should include: sys + ch0 + ch0-2 = 3
	if len(ch0) != 3 {
		t.Fatalf("expected 3 messages for channel 0, got %d", len(ch0))
	}

	ch1 := ms.GetMessagesByChannel(1)
	// Should include: sys + ch1 = 2
	if len(ch1) != 2 {
		t.Fatalf("expected 2 messages for channel 1, got %d", len(ch1))
	}
}

func TestGetMessagesByConversation(t *testing.T) {
	ms := NewMessageStore()
	localID := uint32(42)
	remoteID := uint32(99)
	otherID := uint32(88)

	ms.CreateSystemMessage("sys")

	// DM from remote to local
	ms.CreateIncomingMessage("hi from remote", remoteID, "Remote", localID, 0, time.Now())
	// DM from local to remote
	ms.CreateOutgoingMessage("reply", remoteID, 0, localID, "Me")
	// Broadcast — should NOT appear in conversation
	ms.CreateIncomingMessage("broadcast", remoteID, "Remote", BroadcastAddr, 0, time.Now())
	// DM from other — should NOT appear
	ms.CreateIncomingMessage("other", otherID, "Other", localID, 0, time.Now())

	conv := ms.GetMessagesByConversation(localID, remoteID)
	// sys + "hi from remote" + "reply" = 3
	if len(conv) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(conv))
	}

	// Verify the non-system messages are the right ones.
	nonSys := 0
	for _, m := range conv {
		if m.Type != MessageTypeSystem {
			nonSys++
		}
	}
	if nonSys != 2 {
		t.Fatalf("expected 2 non-system messages, got %d", nonSys)
	}
}

func TestIsBroadcast(t *testing.T) {
	m := Message{RecipientID: BroadcastAddr}
	if !m.IsBroadcast() {
		t.Fatal("expected broadcast")
	}
	m2 := Message{RecipientID: 42}
	if m2.IsBroadcast() {
		t.Fatal("expected NOT broadcast")
	}
}

func TestUpdateMessageStatus(t *testing.T) {
	ms := NewMessageStore()
	msg := ms.CreateOutgoingMessage("test", BroadcastAddr, 0, 1, "Me")

	if !ms.UpdateMessageStatus(msg.ID, MessageStatusSent) {
		t.Fatal("expected update to succeed")
	}

	all := ms.GetAllMessages()
	if all[0].Status != MessageStatusSent {
		t.Fatal("status not updated")
	}

	if ms.UpdateMessageStatus("nonexistent", MessageStatusFailed) {
		t.Fatal("expected update of nonexistent ID to fail")
	}
}

func TestFormatNodeID(t *testing.T) {
	tests := []struct {
		input uint32
		want  string
	}{
		{0, "!00000000"},
		{255, "!000000ff"},
		{0xDEADBEEF, "!deadbeef"},
	}
	for _, tt := range tests {
		got := FormatNodeID(tt.input)
		if got != tt.want {
			t.Errorf("FormatNodeID(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomHex_Unique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		h := randomHex(4)
		if len(h) != 8 {
			t.Fatalf("expected 8 hex chars, got %d", len(h))
		}
		if _, exists := seen[h]; exists {
			t.Fatalf("collision on iteration %d: %q", i, h)
		}
		seen[h] = struct{}{}
	}
}

func TestGenerateMessageID_NonEmpty(t *testing.T) {
	id1 := generateMessageID()
	id2 := generateMessageID()
	if id1 == "" || id2 == "" {
		t.Fatal("IDs should not be empty")
	}
	// They may be equal if called within the same millisecond, but the random
	// suffix makes collisions extremely unlikely.
	if id1 == id2 {
		t.Log("warning: IDs collided, very unlikely but possible")
	}
}

func TestGetAllMessages_Snapshot(t *testing.T) {
	ms := NewMessageStore()
	ms.CreateSystemMessage("one")

	snap := ms.GetAllMessages()
	ms.CreateSystemMessage("two")

	if len(snap) != 1 {
		t.Fatal("snapshot should not be affected by later additions")
	}
}
