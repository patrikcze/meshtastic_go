package protocol

import (
	"log"
	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"
)

// HandleMessageProto processes incoming protobuf messages and updates state or dispatches events.
//
// Parameters:
//   - msg: The protobuf message received from the device.
//   - dispatcher: The event dispatcher responsible for handling events.
//   - state: The state object where node information and configs are stored.
//
// This function decodes the message and performs actions based on its type.
func HandleMessageProto(msg *generated.FromRadio, dispatcher *transport.EventDispatcher, state *transport.State) {
	switch payload := msg.GetPayloadVariant().(type) {
	case *generated.FromRadio_MyInfo:
		state.SetNodeInfo(msg.GetMyInfo())
		log.Printf("Node info received: %+v", msg.GetMyInfo())

	case *generated.FromRadio_Metadata:
		state.SetDeviceMetadata(msg.GetMetadata())
		log.Printf("Device metadata received: %+v", msg.GetMetadata())

	case *generated.FromRadio_NodeInfo:
		node := msg.GetNodeInfo()
		state.AddNode(node)
		log.Printf("Node info added: %+v", node)

	case *generated.FromRadio_Channel:
		HandleChannel(msg.GetChannel())
		state.AddChannel(msg.GetChannel())

	case *generated.FromRadio_Config:
		HandleConfig(msg.GetConfig())
		state.AddConfig(msg.GetConfig())

	case *generated.FromRadio_ModuleConfig:
		cfg := msg.GetModuleConfig()
		state.AddModule(cfg)
		log.Printf("Module config received: %+v", cfg)

	case *generated.FromRadio_Packet:
		packet := msg.GetPacket()
		log.Printf("Packet received: %+v", packet)
		dispatcher.Dispatch(transport.Event{Type: transport.EventMeshPacketReceived, Data: packet})

	case *generated.FromRadio_FileInfo:
		fileInfo := msg.GetFileInfo()
		log.Printf("File info received: %s (%d bytes)", fileInfo.FileName, fileInfo.SizeBytes)

	case *generated.FromRadio_MqttClientProxyMessage:
		mqttMessage := msg.GetMqttClientProxyMessage()
		log.Printf("MQTT Proxy message received: topic: %s, data: %s", mqttMessage.Topic, string(mqttMessage.GetData()))

	case *generated.FromRadio_QueueStatus:
		queueStatus := msg.GetQueueStatus()
		log.Printf("Queue status received: %v", queueStatus)

	default:
		log.Printf("Unknown message type received: %+v", payload)
	}
}
