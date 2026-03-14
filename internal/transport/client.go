package transport

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	meshtastic "meshtastic_go/pkg/generated"

	"google.golang.org/protobuf/proto"
)

var (
	// ErrTimeout is returned when the connection to the radio times out.
	ErrTimeout = errors.New("timeout connecting to radio")
)

const (
	// eventChanSize is the buffer size for the Events channel.
	eventChanSize = 128
	// defaultHopLimit is the default hop count for outgoing packets.
	defaultHopLimit = 3
)

// Client is the single owner of a Meshtastic radio connection.
// After Connect returns, a background goroutine reads packets and emits
// typed RadioEvents on the Events channel for the UI to consume.
type Client struct {
	sc  *StreamConn
	log *slog.Logger

	// Events delivers typed radio events to the consumer (UI layer).
	// The channel is closed when the read goroutine exits.
	Events chan RadioEvent

	State State
}

// State holds all configuration and node data received from the radio.
// All public methods are safe for concurrent access.
type State struct {
	sync.RWMutex
	complete       bool
	configID       uint32
	nodeInfo       *meshtastic.MyNodeInfo
	deviceMetadata *meshtastic.DeviceMetadata
	nodes          []*meshtastic.NodeInfo
	channels       []*meshtastic.Channel
	configs        []*meshtastic.Config
	modules        []*meshtastic.ModuleConfig
}

// ---------- State read accessors (return clones) ----------

// Complete returns true if the initial configuration handshake finished.
func (s *State) Complete() bool {
	s.RLock()
	defer s.RUnlock()
	return s.complete
}

// ConfigID returns the configuration handshake ID.
func (s *State) ConfigID() uint32 {
	s.RLock()
	defer s.RUnlock()
	return s.configID
}

// NodeInfo returns the local node metadata.
func (s *State) NodeInfo() *meshtastic.MyNodeInfo {
	s.RLock()
	defer s.RUnlock()
	if s.nodeInfo == nil {
		return nil
	}
	return proto.Clone(s.nodeInfo).(*meshtastic.MyNodeInfo)
}

// DeviceMetadata returns the device metadata.
func (s *State) DeviceMetadata() *meshtastic.DeviceMetadata {
	s.RLock()
	defer s.RUnlock()
	if s.deviceMetadata == nil {
		return nil
	}
	return proto.Clone(s.deviceMetadata).(*meshtastic.DeviceMetadata)
}

// Nodes returns a cloned list of all known nodes.
func (s *State) Nodes() []*meshtastic.NodeInfo {
	s.RLock()
	defer s.RUnlock()
	out := make([]*meshtastic.NodeInfo, len(s.nodes))
	for i, n := range s.nodes {
		out[i] = proto.Clone(n).(*meshtastic.NodeInfo)
	}
	return out
}

// Channels returns a cloned list of all configured channels.
func (s *State) Channels() []*meshtastic.Channel {
	s.RLock()
	defer s.RUnlock()
	out := make([]*meshtastic.Channel, len(s.channels))
	for i, c := range s.channels {
		out[i] = proto.Clone(c).(*meshtastic.Channel)
	}
	return out
}

// Configs returns a cloned list of all configuration sections.
func (s *State) Configs() []*meshtastic.Config {
	s.RLock()
	defer s.RUnlock()
	out := make([]*meshtastic.Config, len(s.configs))
	for i, c := range s.configs {
		out[i] = proto.Clone(c).(*meshtastic.Config)
	}
	return out
}

// Modules returns a cloned list of all module configurations.
func (s *State) Modules() []*meshtastic.ModuleConfig {
	s.RLock()
	defer s.RUnlock()
	out := make([]*meshtastic.ModuleConfig, len(s.modules))
	for i, m := range s.modules {
		out[i] = proto.Clone(m).(*meshtastic.ModuleConfig)
	}
	return out
}

// ---------- State write methods ----------

// SetComplete marks the configuration phase as done.
func (s *State) SetComplete(v bool) {
	s.Lock()
	defer s.Unlock()
	s.complete = v
}

// SetConfigID stores the random config handshake ID.
func (s *State) SetConfigID(id uint32) {
	s.Lock()
	defer s.Unlock()
	s.configID = id
}

// SetNodeInfo stores the local MyNodeInfo.
func (s *State) SetNodeInfo(info *meshtastic.MyNodeInfo) {
	s.Lock()
	defer s.Unlock()
	s.nodeInfo = info
}

// SetDeviceMetadata stores the device metadata.
func (s *State) SetDeviceMetadata(meta *meshtastic.DeviceMetadata) {
	s.Lock()
	defer s.Unlock()
	s.deviceMetadata = meta
}

// UpsertNode inserts or updates a node by Num.
func (s *State) UpsertNode(node *meshtastic.NodeInfo) {
	s.Lock()
	defer s.Unlock()
	for i, n := range s.nodes {
		if n.Num == node.Num {
			s.nodes[i] = node
			return
		}
	}
	s.nodes = append(s.nodes, node)
}

// AddChannel appends a channel to the list.
func (s *State) AddChannel(ch *meshtastic.Channel) {
	s.Lock()
	defer s.Unlock()
	s.channels = append(s.channels, ch)
}

// AddConfig appends a config section.
func (s *State) AddConfig(cfg *meshtastic.Config) {
	s.Lock()
	defer s.Unlock()
	s.configs = append(s.configs, cfg)
}

// AddModule appends a module config.
func (s *State) AddModule(mod *meshtastic.ModuleConfig) {
	s.Lock()
	defer s.Unlock()
	s.modules = append(s.modules, mod)
}

// ---------- Client construction ----------

// NewClient creates a new Client that owns the given StreamConn.
func NewClient(sc *StreamConn) *Client {
	return &Client{
		log:    slog.Default().WithGroup("client"),
		sc:     sc,
		Events: make(chan RadioEvent, eventChanSize),
	}
}

// ---------- Public API ----------

// MyNodeNum returns the local node number, or 0 if not yet known.
func (c *Client) MyNodeNum() uint32 {
	info := c.State.NodeInfo()
	if info == nil {
		return 0
	}
	return info.MyNodeNum
}

// NodeName returns the long name for a given node number, or "!<hex>" if unknown.
func (c *Client) NodeName(num uint32) string {
	c.State.RLock()
	defer c.State.RUnlock()
	for _, n := range c.State.nodes {
		if n.Num == num && n.User != nil && n.User.LongName != "" {
			return n.User.LongName
		}
	}
	return fmt.Sprintf("!%08x", num)
}

// LocalNodeName returns the long name of the local node, falling back to the hex ID.
func (c *Client) LocalNodeName() string {
	num := c.MyNodeNum()
	if num == 0 {
		return "Unknown"
	}
	return c.NodeName(num)
}

// SendText sends a text message to the specified destination on the given channel.
// Use 0xFFFFFFFF for channel broadcasts.
func (c *Client) SendText(to uint32, channelIndex uint32, message string) error {
	decoded := &meshtastic.Data{
		Portnum:      meshtastic.PortNum_TEXT_MESSAGE_APP,
		Payload:      []byte(message),
		WantResponse: false,
	}

	packet := &meshtastic.MeshPacket{
		To:       to,
		From:     c.MyNodeNum(),
		Channel:  channelIndex,
		HopLimit: defaultHopLimit,
		WantAck:  true,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: decoded,
		},
	}

	toRadio := &meshtastic.ToRadio{
		PayloadVariant: &meshtastic.ToRadio_Packet{
			Packet: packet,
		},
	}

	if err := c.sc.Write(toRadio); err != nil {
		return fmt.Errorf("sending text message: %w", err)
	}
	c.log.Debug("text message sent", "to", to, "channel", channelIndex, "len", len(message))
	return nil
}

// Close shuts down the underlying stream connection.
func (c *Client) Close() error {
	return c.sc.Close()
}

// Connect performs the config handshake and starts the background read goroutine.
// It blocks until the config phase completes or ctx expires.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.sendGetConfig(); err != nil {
		return fmt.Errorf("requesting config: %w", err)
	}

	cfgComplete := make(chan struct{})
	go c.readLoop(cfgComplete)

	select {
	case <-ctx.Done():
		return ErrTimeout
	case <-cfgComplete:
		return nil
	}
}

// ---------- Internal ----------

// sendGetConfig sends WantConfigId to kick off the handshake.
func (c *Client) sendGetConfig() error {
	var r uint32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &r); err != nil {
		return fmt.Errorf("generating random config ID: %w", err)
	}
	c.State.SetConfigID(r)

	msg := &meshtastic.ToRadio{
		PayloadVariant: &meshtastic.ToRadio_WantConfigId{
			WantConfigId: r,
		},
	}
	c.log.Debug("sending want config", "id", r)
	return c.sc.Write(msg)
}

// readLoop continuously reads FromRadio messages, populates State during config,
// and emits typed events after config is complete.
func (c *Client) readLoop(cfgComplete chan struct{}) {
	defer close(c.Events)
	configSignaled := false

	for {
		msg := &meshtastic.FromRadio{}
		if err := c.sc.Read(msg); err != nil {
			c.log.Error("read error", "err", err)
			c.emit(ConnectionEvent{Status: StatusDisconnected, Message: err.Error()})
			return
		}

		switch v := msg.GetPayloadVariant().(type) {
		// --- Config phase packets ---
		case *meshtastic.FromRadio_MyInfo:
			c.State.SetNodeInfo(msg.GetMyInfo())
		case *meshtastic.FromRadio_Metadata:
			c.State.SetDeviceMetadata(msg.GetMetadata())
		case *meshtastic.FromRadio_NodeInfo:
			node := msg.GetNodeInfo()
			c.State.UpsertNode(node)
			c.emit(NodeUpdateEvent{Node: proto.Clone(node).(*meshtastic.NodeInfo)})
		case *meshtastic.FromRadio_Channel:
			c.State.AddChannel(msg.GetChannel())
		case *meshtastic.FromRadio_Config:
			c.State.AddConfig(msg.GetConfig())
		case *meshtastic.FromRadio_ModuleConfig:
			c.State.AddModule(msg.GetModuleConfig())
		case *meshtastic.FromRadio_ConfigCompleteId:
			c.log.Debug("config complete", "id", v.ConfigCompleteId)
			if !configSignaled {
				configSignaled = true
				c.State.SetComplete(true)
				close(cfgComplete)
				c.emit(ConnectionEvent{Status: StatusConnected, Message: "Configuration complete"})
			}

		// --- Runtime packets ---
		case *meshtastic.FromRadio_Packet:
			c.handleMeshPacket(msg.GetPacket())
		case *meshtastic.FromRadio_LogRecord:
			// Device debug logs — ignore in TUI mode.
		case *meshtastic.FromRadio_QueueStatus:
			// Queue status — ignore for now.
		case *meshtastic.FromRadio_Rebooted:
			c.log.Warn("device rebooted")
			c.emit(ConnectionEvent{Status: StatusDisconnected, Message: "Device rebooted"})
		default:
			c.log.Debug("unhandled FromRadio variant", "type", fmt.Sprintf("%T", msg.GetPayloadVariant()))
		}
	}
}

// handleMeshPacket decodes a MeshPacket and emits the appropriate typed event.
func (c *Client) handleMeshPacket(pkt *meshtastic.MeshPacket) {
	if pkt == nil {
		return
	}

	decoded := pkt.GetDecoded()
	if decoded == nil {
		// Encrypted packet we can't decode — skip.
		return
	}

	switch decoded.Portnum {
	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		ts := time.Now()
		if pkt.RxTime > 0 {
			ts = time.Unix(int64(pkt.RxTime), 0)
		}
		c.emit(TextMessageEvent{
			From:       pkt.From,
			To:         pkt.To,
			Channel:    pkt.Channel,
			Content:    string(decoded.Payload),
			SenderName: c.NodeName(pkt.From),
			Timestamp:  ts,
			MessageID:  pkt.Id,
		})

	case meshtastic.PortNum_POSITION_APP:
		pos := &meshtastic.Position{}
		if err := proto.Unmarshal(decoded.Payload, pos); err != nil {
			c.log.Warn("failed to decode position", "err", err)
			return
		}
		c.emit(PositionEvent{From: pkt.From, Position: pos})

	case meshtastic.PortNum_TELEMETRY_APP:
		tel := &meshtastic.Telemetry{}
		if err := proto.Unmarshal(decoded.Payload, tel); err != nil {
			c.log.Warn("failed to decode telemetry", "err", err)
			return
		}
		if dm := tel.GetDeviceMetrics(); dm != nil {
			c.emit(TelemetryEvent{From: pkt.From, Metrics: dm})
		}

	case meshtastic.PortNum_NODEINFO_APP:
		user := &meshtastic.User{}
		if err := proto.Unmarshal(decoded.Payload, user); err != nil {
			c.log.Warn("failed to decode nodeinfo", "err", err)
			return
		}
		ni := &meshtastic.NodeInfo{
			Num:       pkt.From,
			User:      user,
			LastHeard: pkt.RxTime,
			Snr:       pkt.RxSnr,
		}
		c.State.UpsertNode(ni)
		c.emit(NodeUpdateEvent{Node: proto.Clone(ni).(*meshtastic.NodeInfo)})

	default:
		c.log.Debug("unhandled portnum", "portnum", decoded.Portnum, "from", pkt.From)
	}
}

// emit sends an event to the Events channel without blocking.
// If the channel is full the event is dropped (the UI is too slow).
func (c *Client) emit(ev RadioEvent) {
	select {
	case c.Events <- ev:
	default:
		c.log.Warn("event channel full, dropping event", "type", fmt.Sprintf("%T", ev))
	}
}
