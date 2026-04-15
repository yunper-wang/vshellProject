package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

var (
	ErrInvalidFrame  = errors.New("invalid frame format")
	ErrShortFrame    = errors.New("frame too short")
	ErrPayloadTooBig = errors.New("payload exceeds maximum size")
	ErrCRCMismatch   = errors.New("CRC32 mismatch")
)

const (
	FrameHeaderSize  = 7 // Version(1) + Channel(1) + Type(1) + Length(4)
	FrameTrailerSize = 4 // CRC32
	FrameMinSize     = FrameHeaderSize + FrameTrailerSize
	MaxPayloadSize   = 4 * 1024 * 1024 // 4MB
)

// Frame represents a protocol frame
type Frame struct {
	Version uint8
	Channel uint8
	Type    uint8
	Payload []byte
}

// Encode serializes a Frame into bytes
func (f *Frame) Encode() ([]byte, error) {
	if len(f.Payload) > MaxPayloadSize {
		return nil, ErrPayloadTooBig
	}

	buf := new(bytes.Buffer)

	// Write header
	if err := buf.WriteByte(f.Version); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(f.Channel); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(f.Type); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(f.Payload))); err != nil {
		return nil, err
	}

	// Write payload
	if _, err := buf.Write(f.Payload); err != nil {
		return nil, err
	}

	// Calculate and write CRC32
	data := buf.Bytes()
	crc := crc32.ChecksumIEEE(data)
	if err := binary.Write(buf, binary.BigEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode parses bytes into a Frame
func Decode(data []byte) (*Frame, error) {
	if len(data) < FrameMinSize {
		return nil, ErrShortFrame
	}

	// Verify CRC32
	expectedCRC := binary.BigEndian.Uint32(data[len(data)-4:])
	actualCRC := crc32.ChecksumIEEE(data[:len(data)-4])
	if expectedCRC != actualCRC {
		return nil, ErrCRCMismatch
	}

	frame := &Frame{
		Version: data[0],
		Channel: data[1],
		Type:    data[2],
	}

	payloadLen := binary.BigEndian.Uint32(data[3:7])
	if int(payloadLen) > MaxPayloadSize {
		return nil, ErrPayloadTooBig
	}

	if len(data) != FrameMinSize+int(payloadLen) {
		return nil, ErrInvalidFrame
	}

	frame.Payload = make([]byte, payloadLen)
	copy(frame.Payload, data[7:7+payloadLen])

	return frame, nil
}

// NewFrame creates a new frame with default version
func NewFrame(channel, msgType uint8, payload []byte) *Frame {
	return &Frame{
		Version: Version,
		Channel: channel,
		Type:    msgType,
		Payload: payload,
	}
}

// IsControl returns true if this is a control channel frame
func (f *Frame) IsControl() bool {
	return f.Channel == ChannelControl
}

// IsShell returns true if this is a shell channel frame
func (f *Frame) IsShell() bool {
	return f.Channel == ChannelShell
}

// IsFile returns true if this is a file channel frame
func (f *Frame) IsFile() bool {
	return f.Channel == ChannelFile
}

// String returns a human-readable description of the frame
func (f *Frame) String() string {
	return ""
}
