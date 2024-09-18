package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
	"google.golang.org/protobuf/proto"
)

const (
	START1                 = 0x94
	START2                 = 0xC3
	HEADER_LEN             = 4
	MAX_TO_FROM_RADIO_SIZE = 512
)

// StreamConn is a wrapper around a serial connection that provides methods to read and write protobuf messages.
type StreamConn struct {
	conn serial.Port
	mu   sync.Mutex
}

// Close closes the underlying serial connection.
func (sc *StreamConn) Close() error {
	return sc.conn.Close()
}

func NewRadioStreamConn(port serial.Port) *StreamConn {
	return &StreamConn{conn: port}
}

// WriteWake sends 32 bytes to wake the radio
func (sc *StreamConn) WriteWake() error {
	_, err := sc.conn.Write(bytes.Repeat([]byte{START2}, 32))
	if err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond) // Wait 100ms after sending wake
	return nil
}

// NewSerialStreamConn creates a new serial connection with the given device path and mode
func NewSerialStreamConn(devPath string, mode *serial.Mode) (*StreamConn, error) {
	port, err := serial.Open(devPath, mode)
	if err != nil {
		return nil, err
	}
	return NewRadioStreamConn(port), nil
}

// Write a protobuf message
func (sc *StreamConn) Write(in proto.Message) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	data, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("error marshalling proto message: %w", err)
	}

	if len(data) > MAX_TO_FROM_RADIO_SIZE {
		return fmt.Errorf("data length exceeds MTU: %d > %d", len(data), MAX_TO_FROM_RADIO_SIZE)
	}

	err = writeStreamHeader(sc.conn, uint16(len(data)))
	if err != nil {
		return fmt.Errorf("error writing stream header: %w", err)
	}

	_, err = sc.conn.Write(data)
	if err != nil {
		return fmt.Errorf("error writing proto message: %w", err)
	}

	return nil
}

// Helper to write the header
func writeStreamHeader(w io.Writer, dataLen uint16) error {
	header := bytes.NewBuffer(nil)
	header.WriteByte(START1)
	header.WriteByte(START2)
	err := binary.Write(header, binary.BigEndian, dataLen)
	if err != nil {
		return fmt.Errorf("writing length to buffer: %w", err)
	}
	_, err = w.Write(header.Bytes())
	return err
}

// Read a protobuf message
func (sc *StreamConn) Read(out proto.Message) error {
	header := make([]byte, HEADER_LEN)
	for {
		_, err := io.ReadFull(sc.conn, header)
		if err != nil {
			return fmt.Errorf("error reading header: %w", err)
		}

		if header[0] != START1 || header[1] != START2 {
			continue
		}

		length := int(header[2])<<8 | int(header[3])
		if length > MAX_TO_FROM_RADIO_SIZE {
			continue
		}

		message := make([]byte, length)
		_, err = io.ReadFull(sc.conn, message)
		if err != nil {
			return fmt.Errorf("error reading message body: %w", err)
		}

		return proto.Unmarshal(message, out)
	}
}
