package protocol

// Protocol package provides the wire protocol for vshell
// Frame structure:
// [Version:1][Channel:1][Type:1][Length:4][Payload:N][CRC32:4]
//
// Channel multiplexing:
// - Channel 0: Control messages (Hello, Heartbeat, Error)
// - Channel 1: Shell/PTY data
// - Channel 2: File transfer

import (
	"net"
)

// Conn wraps a net.Conn with protocol framing
type Conn struct {
	net.Conn
	*Mux
}

// NewConn wraps a net.Conn with protocol multiplexing
func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn: conn,
		Mux:  NewMux(conn, conn),
	}
}

// UpgradeConn wraps an existing net.Conn (for TLSListener)
func UpgradeConn(conn net.Conn) *Conn {
	return NewConn(conn)
}

// Protocol handshake helpers

// ClientHandshake performs client-side handshake
func ClientHandshake(conn *Conn, clientInfo *ClientInfo) (*Ok, error) {
	hello := NewHello(Version, []string{"shell", "pty", "file"}, clientInfo)
	payload, err := hello.MarshalBinary()
	if err != nil {
		return nil, err
	}

	f := NewFrame(ChannelControl, TypeHello, payload)
	if err := conn.WriteFrame(f); err != nil {
		return nil, err
	}

	resp, err := conn.ReadFrame()
	if err != nil {
		return nil, err
	}

	switch resp.Type {
	case TypeOk:
		ok := &Ok{}
		if err := ok.UnmarshalBinary(resp.Payload); err != nil {
			return nil, err
		}
		return ok, nil
	case TypeError:
		var e Error
		if err := e.UnmarshalBinary(resp.Payload); err != nil {
			return nil, err
		}
		return nil, &e
	default:
		return nil, ErrInvalidFrame
	}
}

// ServerHandshake performs server-side handshake
func ServerHandshake(conn *Conn, serverInfo *ServerInfo) (*Hello, *Ok, error) {
	f, err := conn.ReadFrame()
	if err != nil {
		return nil, nil, err
	}

	if f.Type != TypeHello {
		return nil, nil, ErrInvalidFrame
	}

	hello := &Hello{}
	if err := hello.UnmarshalBinary(f.Payload); err != nil {
		return nil, nil, err
	}

	if hello.Version != Version {
		errResp := NewError(ErrInvalidMsg, "unsupported protocol version")
		payload, _ := errResp.MarshalBinary()
		conn.WriteFrame(NewFrame(ChannelControl, TypeError, payload))
		return hello, nil, errResp
	}

	ok := NewOk(Version, []string{"shell", "pty", "file"}, "", serverInfo)
	return hello, ok, nil
}

// SendOk sends an Ok response
func SendOk(conn *Conn, ok *Ok) error {
	payload, err := ok.MarshalBinary()
	if err != nil {
		return err
	}
	return conn.WriteFrame(NewFrame(ChannelControl, TypeOk, payload))
}

// SendError sends an Error response
func SendError(conn *Conn, code int, message string) error {
	e := NewError(code, message)
	payload, err := e.MarshalBinary()
	if err != nil {
		return err
	}
	return conn.WriteFrame(NewFrame(ChannelControl, TypeError, payload))
}

// Message types for protocol
type (
	// Empty struct for compatibility
	Empty struct{}
)
