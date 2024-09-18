package protocol

import (
	"log"
	"meshtastic_go/pkg/generated"
)

// HandleConfig processes and logs configuration data in a structured way
func HandleConfig(cfg *generated.Config) {
	if cfg.GetPosition() != nil {
		pos := cfg.GetPosition()
		log.Printf("Position Config: \n- Broadcast Interval: %d secs\n- Smart Enabled: %v\n- Min Distance: %d meters\n- Min Interval: %d secs",
			pos.PositionBroadcastSecs, pos.PositionBroadcastSmartEnabled, pos.BroadcastSmartMinimumDistance, pos.BroadcastSmartMinimumIntervalSecs)
	}

	if cfg.GetPower() != nil {
		power := cfg.GetPower()
		log.Printf("Power Config: \n- Bluetooth Wait Time: %d secs\n- SDS Time: %d secs\n- LS Time: %d secs\n- Min Wake Time: %d secs",
			power.WaitBluetoothSecs, power.SdsSecs, power.LsSecs, power.MinWakeSecs)
	}

	if cfg.GetNetwork() != nil {
		network := cfg.GetNetwork()
		log.Printf("Network Config: \n- NTP Server: %s", network.NtpServer)
	}

	if cfg.GetDisplay() != nil {
		display := cfg.GetDisplay()
		log.Printf("Display Config: \n- Screen On Time: %d secs\n- Wake on Tap/Motion: %v",
			display.ScreenOnSecs, display.WakeOnTapOrMotion)
	}

	if cfg.GetLora() != nil {
		lora := cfg.GetLora()
		log.Printf("LoRa Config: \n- Use Preset: %v\n- Region: %s\n- Hop Limit: %d\n- TX Power: %d dBm",
			lora.UsePreset, lora.Region, lora.HopLimit, lora.TxPower)
	}

	if cfg.GetBluetooth() != nil {
		bluetooth := cfg.GetBluetooth()
		log.Printf("Bluetooth Config: \n- Enabled: %v\n- Mode: %s\n- Fixed PIN: %d",
			bluetooth.Enabled, bluetooth.Mode.String(), bluetooth.FixedPin)
	}
}
