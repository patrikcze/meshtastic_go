package serial

import (
	"go.bug.st/serial"
)

const (
	portSpeed = 115200 //921600
	dataBits  = 8
)

// Connect returns a list of available serial ports.
func Connect(port string) (serial.Port, error) {
	mode := &serial.Mode{
		BaudRate: portSpeed,
		DataBits: dataBits,
	}
	p, err := serial.Open(port, mode)
	if err != nil {
		return nil, err
	}
	return p, nil
}
