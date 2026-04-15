package protocol

import (
	"io"
	"sync"
)

// Mux handles framing/unframing over a connection
type Mux struct {
	writer io.Writer
	reader io.Reader
	mu     sync.Mutex
}

// NewMux creates a new multiplexed connection
func NewMux(w io.Writer, r io.Reader) *Mux {
	return &Mux{
		writer: w,
		reader: r,
	}
}

// WriteFrame sends a frame with proper framing
func (m *Mux) WriteFrame(f *Frame) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := f.Encode()
	if err != nil {
		return err
	}

	// Simple framing: length prefix (4 bytes) + data
	buf := make([]byte, 4+len(data))
	buf[0] = byte(len(data) >> 24)
	buf[1] = byte(len(data) >> 16)
	buf[2] = byte(len(data) >> 8)
	buf[3] = byte(len(data))
	copy(buf[4:], data)

	_, err = m.writer.Write(buf)
	return err
}

// ReadFrame reads and parses a frame
func (m *Mux) ReadFrame() (*Frame, error) {
	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(m.reader, lenBuf); err != nil {
		return nil, err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	if length > MaxPayloadSize+FrameMinSize {
		return nil, ErrPayloadTooBig
	}

	// Read frame data
	data := make([]byte, length)
	if _, err := io.ReadFull(m.reader, data); err != nil {
		return nil, err
	}

	return Decode(data)
}

// Close closes the underlying connection if it implements io.Closer
func (m *Mux) Close() error {
	if c, ok := m.writer.(io.Closer); ok {
		return c.Close()
	}
	if c, ok := m.reader.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// ChannelReader provides an io.Reader interface for a specific channel
// (placeholder for future channel-based multiplexing)
type ChannelReader struct {
	Channel uint8
	mux     *Mux
	ch      chan []byte
	buffer  []byte
}

// Read implements io.Reader
// For now, this is a placeholder for future channel-based I/O
func (cr *ChannelReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

// ChannelWriter provides an io.Writer interface for a specific channel
type ChannelWriter struct {
	Channel uint8
	Type    uint8
	mux     *Mux
	mu      sync.Mutex
}

// Write implements io.Writer
func (cw *ChannelWriter) Write(p []byte) (n int, err error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	f := NewFrame(cw.Channel, cw.Type, p)
	if err := cw.mux.WriteFrame(f); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ChannelClose marks end of channel data
func (cw *ChannelWriter) Close() error {
	f := NewFrame(cw.Channel, cw.Type|0x80, nil) // Set EOF bit
	return cw.mux.WriteFrame(f)
}
