package protocol

import (
	"log"
	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"
)

// SendTextMessage sends a text message to a specific receiver over the given stream connection.
// Parameters:
//   - streamConn: The connection stream used to send the message.
//   - to: The recipient node ID.
//   - from: The sender node ID.
//   - message: The actual text message to be sent.
//   - response: Set to true if you want a response from the recipient.
//
// Returns:
//   - An error if the message sending fails, otherwise nil.
func SendTextMessage(streamConn *transport.StreamConn, to uint32, from uint32, message string, response bool) error {
	// Construct the inner Decoded message (assuming there's a "Data" or similar type wrapping the Portnum and Payload)
	decoded := &generated.Data{
		Portnum:      generated.PortNum_TEXT_MESSAGE_APP, // Set the port number for text message
		Payload:      []byte(message),                    // The actual text message payload
		WantResponse: response,                           // Set to true if you want a response

	}

	// Create the MeshPacket protobuf message and set its Decoded field
	meshPacket := &generated.MeshPacket{
		To:       to,   // Receiver ID (use a valid receiver node ID)
		From:     from, // Sender ID (use your own node ID)
		HopLimit: 3,
		PayloadVariant: &generated.MeshPacket_Decoded{
			Decoded: decoded,
		},
	}

	// Create the ToRadio message with the MeshPacket
	toRadio := &generated.ToRadio{
		PayloadVariant: &generated.ToRadio_Packet{
			Packet: meshPacket,
		},
	}
	// TODO: for debugging only remove
	log.Printf("Sending text message: %v", toRadio)
	// Send the ToRadio message over the stream
	err := streamConn.Write(toRadio)
	if err != nil {
		log.Printf("Failed to send text message: %v", err)
		return err
	}

	log.Printf("Text message sent to %d: %s", to, message)
	return nil
}
