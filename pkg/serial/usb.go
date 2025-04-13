package serial

type usbDevice struct {
	VID string
	PID string
}

// knownDevices is a list of known Meshtastic device USB VID/PID combinations
var knownDevices = []usbDevice{
	// rak4631_19003
	{VID: "239A", PID: "8029"},
	// CP210x UART Bridge (commonly found on Heltec and other devices)
	{VID: "10C4", PID: "EA60"},
	// LILYGO TTGO T-Beam
	{VID: "1A86", PID: "55D4"},
	// Heltec devices
	{VID: "303A", PID: "1001"},
	// RAK devices
	{VID: "0483", PID: "5740"},
	// CH340 Serial
	{VID: "1A86", PID: "7523"},
}
