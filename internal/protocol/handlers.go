package protocol

import (
	"log"

	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"
)

// SendConfigRequest sends a configuration request to the radio.
//
// Parameters:
//   - streamConn: The StreamConn used to send the message.
//   - configID: The configuration ID (you can use rand.Uint32() to generate one).
//
// Returns:
//   - An error if the request fails, otherwise nil.
func SendConfigRequest(streamConn *transport.StreamConn, configID uint32) error {
	// Construct the ToRadio message with the WantConfigId payload
	toRadio := &generated.ToRadio{
		PayloadVariant: &generated.ToRadio_WantConfigId{
			WantConfigId: configID,
		},
	}

	// Log the config request for debugging
	log.Printf("Sending config request with ID: %d", configID)

	// Marshal and send the request
	err := streamConn.Write(toRadio)
	if err != nil {
		log.Printf("Failed to send config request: %v", err)
		return err
	}

	log.Printf("Config request sent successfully")
	return nil
}

// HandleMeshPacketReceived processes incoming MeshPacket events
func HandleMeshPacketReceived(event transport.Event) {
	packet, ok := event.Data.(*generated.MeshPacket)
	if !ok {
		log.Println("Failed to cast event data to MeshPacket")
		return
	}

	log.Printf("Received packet from %d to %d, message ID: %d", packet.From, packet.To, packet.Id)

	decoded := packet.GetDecoded()
	if decoded != nil {
		switch decoded.Portnum {
		case generated.PortNum_TEXT_MESSAGE_APP:
			log.Printf("Text message received: %s", string(decoded.Payload))
		case generated.PortNum_POSITION_APP:
			log.Printf("Position message received: %v", decoded.Payload)
		case generated.PortNum_TELEMETRY_APP:
			log.Printf("Telemetry message received: %v", decoded.Payload)
		// Add more cases for different port numbers as needed
		default:
			// Silently skip unknown port numbers
			return
		}
	} else {
		log.Printf("Received packet with no decoded data")
	}
}
