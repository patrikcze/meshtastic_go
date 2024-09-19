package serial

import (
	"go.bug.st/serial"
)

const (
	PORT_SPEED = 115200 //921600
	DATA_BITS  = 8
)

func Connect(port string) (serial.Port, error) {
	mode := &serial.Mode{
		BaudRate: PORT_SPEED,
		DataBits: DATA_BITS,
	}
	p, err := serial.Open(port, mode)
	if err != nil {
		return nil, err
	}
	return p, nil
}
