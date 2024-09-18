package protocol

import (
	"meshtastic_go/pkg/generated"
)

type State struct {
	nodeInfo       *generated.MyNodeInfo
	deviceMetadata *generated.DeviceMetadata
	nodes          []*generated.NodeInfo
	channels       []*generated.Channel
	configs        []*generated.Config
	modules        []*generated.ModuleConfig
}

func (s *State) SetNodeInfo(nodeInfo *generated.MyNodeInfo) {
	s.nodeInfo = nodeInfo
}

func (s *State) SetDeviceMetadata(deviceMetadata *generated.DeviceMetadata) {
	s.deviceMetadata = deviceMetadata
}

func (s *State) AddNode(node *generated.NodeInfo) {
	s.nodes = append(s.nodes, node)
}

func (s *State) AddChannel(channel *generated.Channel) {
	s.channels = append(s.channels, channel)
}

func (s *State) AddConfig(config *generated.Config) {
	s.configs = append(s.configs, config)
}

func (s *State) AddModule(module *generated.ModuleConfig) {
	s.modules = append(s.modules, module)
}
