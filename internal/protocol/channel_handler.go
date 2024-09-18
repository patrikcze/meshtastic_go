package protocol

import (
	"log"
)

// LogChannels logs all channels stored in the state
func (s *State) LogChannels() {
	log.Println("List of Channels:")
	for _, channel := range s.channels {
		settings := channel.GetSettings()
		log.Printf("- Index: %d", channel.GetIndex())
		log.Printf("  Name: %s", settings.GetName())
		log.Printf("  Uplink Enabled: %v", settings.GetUplinkEnabled())
		log.Printf("  Downlink Enabled: %v", settings.GetDownlinkEnabled())
		log.Printf("  Position Precision: %d", settings.GetModuleSettings().GetPositionPrecision())
		log.Println("-----")
	}
}
