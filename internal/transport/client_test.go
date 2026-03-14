package transport

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	meshtastic "meshtastic_go/pkg/generated"

	"google.golang.org/protobuf/proto"
)

// newTestClient creates a Client with a valid logger and buffered Events channel.
func newTestClient(bufSize int) *Client {
	return &Client{
		log:    slog.Default().WithGroup("test"),
		Events: make(chan RadioEvent, bufSize),
	}
}

// ---------- State tests ----------

func TestUpsertNode_Insert(t *testing.T) {
	var s State
	node := &meshtastic.NodeInfo{Num: 100, User: &meshtastic.User{LongName: "Alice"}}
	s.UpsertNode(node)

	nodes := s.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].User.LongName != "Alice" {
		t.Fatalf("expected Alice, got %s", nodes[0].User.LongName)
	}
}

func TestUpsertNode_Update(t *testing.T) {
	var s State
	s.UpsertNode(&meshtastic.NodeInfo{Num: 100, User: &meshtastic.User{LongName: "Alice"}})
	s.UpsertNode(&meshtastic.NodeInfo{Num: 100, User: &meshtastic.User{LongName: "Alice Updated"}})

	nodes := s.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after upsert, got %d", len(nodes))
	}
	if nodes[0].User.LongName != "Alice Updated" {
		t.Fatalf("expected 'Alice Updated', got %s", nodes[0].User.LongName)
	}
}

func TestUpsertNode_Concurrent(t *testing.T) {
	var s State
	var wg sync.WaitGroup

	for i := uint32(0); i < 50; i++ {
		wg.Add(1)
		go func(num uint32) {
			defer wg.Done()
			s.UpsertNode(&meshtastic.NodeInfo{Num: num, User: &meshtastic.User{LongName: "node"}})
		}(i)
	}
	wg.Wait()

	nodes := s.Nodes()
	if len(nodes) != 50 {
		t.Fatalf("expected 50 nodes, got %d", len(nodes))
	}
}

func TestState_Accessors_NilSafe(t *testing.T) {
	var s State
	if s.NodeInfo() != nil {
		t.Fatal("expected nil NodeInfo")
	}
	if s.DeviceMetadata() != nil {
		t.Fatal("expected nil DeviceMetadata")
	}
	if len(s.Nodes()) != 0 {
		t.Fatal("expected empty Nodes")
	}
}

func TestState_CloneIsolation(t *testing.T) {
	var s State
	s.UpsertNode(&meshtastic.NodeInfo{Num: 1, User: &meshtastic.User{LongName: "Original"}})

	nodes := s.Nodes()
	nodes[0].User.LongName = "Mutated"

	fresh := s.Nodes()
	if fresh[0].User.LongName != "Original" {
		t.Fatal("clone isolation violated: mutation leaked back to state")
	}
}

// ---------- Client.NodeName / LocalNodeName tests ----------

func TestNodeName_Known(t *testing.T) {
	c := newTestClient(1)
	c.State.UpsertNode(&meshtastic.NodeInfo{Num: 42, User: &meshtastic.User{LongName: "Bob"}})

	name := c.NodeName(42)
	if name != "Bob" {
		t.Fatalf("expected 'Bob', got %q", name)
	}
}

func TestNodeName_Unknown(t *testing.T) {
	c := newTestClient(1)
	name := c.NodeName(255)
	if name != "!000000ff" {
		t.Fatalf("expected '!000000ff', got %q", name)
	}
}

func TestLocalNodeName_NoInfo(t *testing.T) {
	c := newTestClient(1)
	if c.LocalNodeName() != "Unknown" {
		t.Fatalf("expected 'Unknown', got %q", c.LocalNodeName())
	}
}

// ---------- handleMeshPacket tests ----------

func TestHandleMeshPacket_TextMessage(t *testing.T) {
	c := newTestClient(10)
	c.State.UpsertNode(&meshtastic.NodeInfo{Num: 1, User: &meshtastic.User{LongName: "Alice"}})

	pkt := &meshtastic.MeshPacket{
		From:    1,
		To:      0xFFFFFFFF,
		Channel: 0,
		RxTime:  1700000000,
		Id:      999,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: &meshtastic.Data{
				Portnum: meshtastic.PortNum_TEXT_MESSAGE_APP,
				Payload: []byte("Hello mesh!"),
			},
		},
	}

	c.handleMeshPacket(pkt)

	select {
	case ev := <-c.Events:
		txt, ok := ev.(TextMessageEvent)
		if !ok {
			t.Fatalf("expected TextMessageEvent, got %T", ev)
		}
		if txt.Content != "Hello mesh!" {
			t.Fatalf("wrong content: %q", txt.Content)
		}
		if txt.SenderName != "Alice" {
			t.Fatalf("wrong sender name: %q", txt.SenderName)
		}
		if txt.From != 1 || txt.To != 0xFFFFFFFF || txt.Channel != 0 {
			t.Fatalf("wrong routing: from=%d to=%d ch=%d", txt.From, txt.To, txt.Channel)
		}
		if txt.MessageID != 999 {
			t.Fatalf("wrong message ID: %d", txt.MessageID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no event emitted")
	}
}

func TestHandleMeshPacket_NilDecoded(t *testing.T) {
	c := newTestClient(10)

	// Encrypted packet — should be silently skipped.
	pkt := &meshtastic.MeshPacket{
		From: 1,
		PayloadVariant: &meshtastic.MeshPacket_Encrypted{
			Encrypted: []byte{0x01, 0x02},
		},
	}
	c.handleMeshPacket(pkt)

	select {
	case ev := <-c.Events:
		t.Fatalf("did not expect event for encrypted packet, got %T", ev)
	case <-time.After(50 * time.Millisecond):
		// OK — no event.
	}
}

func TestHandleMeshPacket_NodeInfo(t *testing.T) {
	c := newTestClient(10)

	user := &meshtastic.User{LongName: "NewNode", ShortName: "NN"}
	payload, _ := proto.Marshal(user)

	pkt := &meshtastic.MeshPacket{
		From:   77,
		RxTime: 1700000000,
		RxSnr:  5.5,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: &meshtastic.Data{
				Portnum: meshtastic.PortNum_NODEINFO_APP,
				Payload: payload,
			},
		},
	}
	c.handleMeshPacket(pkt)

	select {
	case ev := <-c.Events:
		nu, ok := ev.(NodeUpdateEvent)
		if !ok {
			t.Fatalf("expected NodeUpdateEvent, got %T", ev)
		}
		if nu.Node.Num != 77 || nu.Node.User.LongName != "NewNode" {
			t.Fatalf("unexpected node: %+v", nu.Node)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no event emitted")
	}

	// Also verify state was updated.
	name := c.NodeName(77)
	if name != "NewNode" {
		t.Fatalf("expected state to contain 'NewNode', got %q", name)
	}
}

func TestEmit_ChannelFull(t *testing.T) {
	c := newTestClient(1)
	// Fill the channel.
	c.emit(ConnectionEvent{Status: StatusConnected})
	// This should not block — it drops silently.
	c.emit(ConnectionEvent{Status: StatusDisconnected})

	ev := <-c.Events
	if ce, ok := ev.(ConnectionEvent); !ok || ce.Status != StatusConnected {
		t.Fatal("expected first event to be StatusConnected")
	}
}

// ---------- fakeStreamConn helper for SendText test ----------

// fakeConn captures bytes written and returns a canned response on read.
type fakeConn struct {
	written bytes.Buffer
	readBuf bytes.Buffer
	mu      sync.Mutex
}

func (f *fakeConn) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readBuf.Len() == 0 {
		return 0, io.EOF
	}
	return f.readBuf.Read(p)
}

func (f *fakeConn) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written.Write(p)
}

func (f *fakeConn) Close() error { return nil }

func TestSendText_WritesToStream(t *testing.T) {
	fc := &fakeConn{}
	sc := NewRadioStreamConn(fc)
	c := NewClient(sc)
	c.State.SetNodeInfo(&meshtastic.MyNodeInfo{MyNodeNum: 42})

	err := c.SendText(0xFFFFFFFF, 0, "test message")
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// Verify something was written (stream header + protobuf).
	data := fc.written.Bytes()
	if len(data) < 4 {
		t.Fatalf("expected data written, got %d bytes", len(data))
	}

	// Verify stream header magic bytes.
	if data[0] != Start1 || data[1] != Start2 {
		t.Fatalf("bad magic: %02x %02x", data[0], data[1])
	}

	// Verify we can unmarshal the protobuf.
	pbLen := binary.BigEndian.Uint16(data[2:4])
	toRadio := &meshtastic.ToRadio{}
	if err := proto.Unmarshal(data[4:4+pbLen], toRadio); err != nil {
		t.Fatalf("failed to unmarshal ToRadio: %v", err)
	}

	pkt := toRadio.GetPacket()
	if pkt == nil {
		t.Fatal("expected Packet payload")
	}
	if pkt.From != 42 || pkt.To != 0xFFFFFFFF {
		t.Fatalf("wrong from/to: %d/%d", pkt.From, pkt.To)
	}
	decoded := pkt.GetDecoded()
	if decoded == nil || string(decoded.Payload) != "test message" {
		t.Fatalf("wrong payload: %v", decoded)
	}
}
