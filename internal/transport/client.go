package transport

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"meshtastic_go/pkg/generated"
	meshtastic "meshtastic_go/pkg/generated"

	"google.golang.org/protobuf/proto"
)

var (
	// ErrTimeout is returned when the connection to the radio times out.
	ErrTimeout = errors.New("timeout connecting to radio")
)

// HandlerFunc is a function that handles a protobuf message.
type HandlerFunc func(message proto.Message)

// Client is a client for interacting with a Meshtastic radio.
type Client struct {
	sc       *StreamConn
	handlers *HandlerRegistry
	log      *slog.Logger

	State State
}

// State represents the state of the client.
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

// Complete returns true if the configuration is complete.
func (s *State) Complete() bool {
	s.RLock()
	defer s.RUnlock()
	return s.complete
}

// ConfigID returns the configuration ID.
func (s *State) ConfigID() uint32 {
	s.RLock()
	defer s.RUnlock()
	return s.configID
}

// NodeInfo returns the node information.
func (s *State) NodeInfo() *meshtastic.MyNodeInfo {
	s.RLock()
	defer s.RUnlock()
	return s.nodeInfo
}

// DeviceMetadata returns the device metadata.
func (s *State) DeviceMetadata() *meshtastic.DeviceMetadata {
	s.RLock()
	defer s.RUnlock()
	return proto.Clone(s.deviceMetadata).(*meshtastic.DeviceMetadata)
}

// Nodes returns the list of nodes.
func (s *State) Nodes() []*meshtastic.NodeInfo {
	s.RLock()
	defer s.RUnlock()
	var nodeInfos []*meshtastic.NodeInfo
	for _, n := range s.nodes {
		nodeInfos = append(nodeInfos, proto.Clone(n).(*meshtastic.NodeInfo))
	}
	return nodeInfos
}

// Channels returns the list of channels.
func (s *State) Channels() []*meshtastic.Channel {
	s.RLock()
	defer s.RUnlock()
	var channels []*meshtastic.Channel
	for _, n := range s.channels {
		channels = append(channels, proto.Clone(n).(*meshtastic.Channel))
	}
	return channels
}

// Configs returns the list of configurations.
func (s *State) Configs() []*meshtastic.Config {
	s.RLock()
	defer s.RUnlock()
	var configs []*meshtastic.Config
	for _, n := range s.configs {
		configs = append(configs, proto.Clone(n).(*meshtastic.Config))
	}
	return configs
}

// Modules returns the list of modules.
func (s *State) Modules() []*meshtastic.ModuleConfig {
	s.RLock()
	defer s.RUnlock()
	var configs []*meshtastic.ModuleConfig
	for _, n := range s.modules {
		configs = append(configs, proto.Clone(n).(*meshtastic.ModuleConfig))
	}
	return configs
}

// SetComplete sets the configuration complete flag.
func (s *State) SetComplete(complete bool) {
	s.Lock()
	defer s.Unlock()
	s.complete = complete
}

// SetConfigID sets the configuration ID.
func (s *State) SetConfigID(configID uint32) {
	s.Lock()
	defer s.Unlock()
	s.configID = configID
}

// SetNodeInfo sets the node information.
func (s *State) SetNodeInfo(nodeInfo *meshtastic.MyNodeInfo) {
	s.Lock()
	defer s.Unlock()
	s.nodeInfo = nodeInfo
}

// SetDeviceMetadata sets the device metadata.
func (s *State) SetDeviceMetadata(deviceMetadata *meshtastic.DeviceMetadata) {
	s.Lock()
	defer s.Unlock()
	s.deviceMetadata = deviceMetadata
}

// AddNode adds a node to the list of nodes.
func (s *State) AddNode(node *meshtastic.NodeInfo) {
	s.Lock()
	defer s.Unlock()
	s.nodes = append(s.nodes, node)
}

// AddChannel adds a channel to the list of channels.
func (s *State) AddChannel(channel *meshtastic.Channel) {
	s.Lock()
	defer s.Unlock()
	s.channels = append(s.channels, channel)
}

// AddConfig adds a configuration to the list of configurations.
func (s *State) AddConfig(config *meshtastic.Config) {
	s.Lock()
	defer s.Unlock()
	s.configs = append(s.configs, config)
}

// AddModule adds a module to the list of modules.
func (s *State) AddModule(module *meshtastic.ModuleConfig) {
	s.Lock()
	defer s.Unlock()
	s.modules = append(s.modules, module)
}

// NewClient creates a new client.
func NewClient(sc *StreamConn, errorOnNoHandler bool) *Client {
	return &Client{
		// TODO: allow consumer to specify logger
		log:      slog.Default().WithGroup("client"),
		sc:       sc,
		handlers: NewHandlerRegistry(errorOnNoHandler),
	}
}

// sendGetConfig sends a GetConfig message to the radio.
func (c *Client) sendGetConfig() error {
	var r uint32
	// Generate a random uint32 using crypto/rand
	if err := binary.Read(rand.Reader, binary.LittleEndian, &r); err != nil {
		return fmt.Errorf("failed to generate random config ID: %w", err)
	}

	c.State.configID = r
	msg := &generated.ToRadio{
		PayloadVariant: &generated.ToRadio_WantConfigId{
			WantConfigId: r,
		},
	}
	c.log.Debug("sending want config", "id", r)
	if err := c.sc.Write(msg); err != nil {
		return fmt.Errorf("writing want config command: %w", err)
	}
	c.log.Debug("sent want config")
	return nil
}

// Handle registers a handler for a protobuf message.
func (c *Client) Handle(kind proto.Message, handler MessageHandler) {
	c.handlers.RegisterHandler(kind, handler)
}

// SendToRadio sends a message to the radio.
func (c *Client) SendToRadio(msg *meshtastic.ToRadio) error {
	return c.sc.Write(msg)
}

// Connect connects to the radio.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.sendGetConfig(); err != nil {
		return fmt.Errorf("requesting config: %w", err)
	}
	cfgComplete := make(chan struct{})
	go func() {
		for {
			msg := &meshtastic.FromRadio{}
			err := c.sc.Read(msg)
			if err != nil {
				c.log.Error("error reading from radio", "err", err)
				continue
			}
			c.log.Debug("received message from radio", "msg", msg)
			var variant proto.Message
			switch msg.GetPayloadVariant().(type) {
			// These pbufs all get sent upon initial connection to the node
			case *meshtastic.FromRadio_MyInfo:
				c.State.SetNodeInfo(msg.GetMyInfo())
				variant = c.State.nodeInfo
			case *meshtastic.FromRadio_Metadata:
				c.State.SetDeviceMetadata(msg.GetMetadata())
				variant = c.State.deviceMetadata
			case *meshtastic.FromRadio_NodeInfo:
				node := msg.GetNodeInfo()
				c.State.AddNode(node)
				variant = node
			case *meshtastic.FromRadio_Channel:
				channel := msg.GetChannel()
				c.State.AddChannel(channel)
				variant = channel
			case *meshtastic.FromRadio_Config:
				cfg := msg.GetConfig()
				c.State.AddConfig(cfg)
				variant = cfg
			case *meshtastic.FromRadio_ModuleConfig:
				cfg := msg.GetModuleConfig()
				c.State.AddModule(cfg)
				variant = cfg
			case *meshtastic.FromRadio_ConfigCompleteId:
				// logged here because it's not an actual proto.Message that we can call handlers on
				c.log.Debug("config complete")
				if !c.State.Complete() {
					close(cfgComplete)
				}
				c.State.SetComplete(true)
				continue
				// below are packets not part of initial connection

			case *meshtastic.FromRadio_LogRecord:
				variant = msg.GetLogRecord()
			case *meshtastic.FromRadio_MqttClientProxyMessage:
				variant = msg.GetMqttClientProxyMessage()
			case *meshtastic.FromRadio_QueueStatus:
				variant = msg.GetQueueStatus()
			case *meshtastic.FromRadio_Rebooted:
				// true if radio just rebooted
				// logged here because it's not an actual proto.Message that we can call handlers on
				c.log.Debug("rebooted", "rebooted", msg.GetRebooted())

				continue
			case *meshtastic.FromRadio_XmodemPacket:
				variant = msg.GetXmodemPacket()
			case *meshtastic.FromRadio_Packet:
				variant = msg.GetPacket()
			default:
				c.log.Warn("unhandled protobuf from radio")
			}

			if !c.State.Complete() {
				continue
			}
			err = c.handlers.HandleMessage(variant)
			if err != nil {
				c.log.Error("error handling message", "err", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ErrTimeout
		case <-cfgComplete:
			return nil
		}
	}
}
