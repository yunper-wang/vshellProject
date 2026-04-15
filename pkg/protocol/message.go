package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Serializable interface for message types
type Serializable interface {
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

// Message is the base interface for all protocol messages
type Message interface {
	GetType() uint8
	GetChannel() uint8
}

// baseMessage holds common fields
type baseMessage struct {
	ChannelID uint8
	TypeID    uint8
}

func (m baseMessage) GetType() uint8    { return m.TypeID }
func (m baseMessage) GetChannel() uint8 { return m.ChannelID }

// Hello is sent by client on connection
type Hello struct {
	baseMessage
	Version  uint8
	Features []string
	Client   *ClientInfo
}

type ClientInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func NewHello(version uint8, features []string, client *ClientInfo) *Hello {
	return &Hello{
		baseMessage: baseMessage{ChannelID: ChannelControl, TypeID: 0x01},
		Version:     version,
		Features:    features,
		Client:      client,
	}
}

func (h Hello) MarshalBinary() ([]byte, error) {
	return json.Marshal(h)
}

func (h *Hello) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, h)
}

// Ok is server response to Hello
type Ok struct {
	baseMessage
	Version   uint8
	Features  []string
	SessionID string
	Server    *ServerInfo
}

type ServerInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func NewOk(version uint8, features []string, sessionID string, server *ServerInfo) *Ok {
	return &Ok{
		baseMessage: baseMessage{ChannelID: ChannelControl, TypeID: 0x02},
		Version:     version,
		Features:    features,
		SessionID:   sessionID,
		Server:      server,
	}
}

func (o Ok) MarshalBinary() ([]byte, error) {
	return json.Marshal(o)
}

func (o *Ok) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, o)
}

// Heartbeat for keepalive
type Heartbeat struct {
	baseMessage
	Timestamp int64
}

func NewHeartbeat() *Heartbeat {
	return &Heartbeat{
		baseMessage: baseMessage{ChannelID: ChannelControl, TypeID: TypeHeartbeat},
		Timestamp:   time.Now().UnixNano(),
	}
}

func (h Heartbeat) MarshalBinary() ([]byte, error) {
	return json.Marshal(h)
}

func (h *Heartbeat) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, h)
}

// Error message
type Error struct {
	baseMessage
	Code    int
	Message string
}

func NewError(code int, message string) *Error {
	return &Error{
		baseMessage: baseMessage{ChannelID: ChannelControl, TypeID: 0xFF},
		Code:        code,
		Message:     message,
	}
}

func (e Error) MarshalBinary() ([]byte, error) {
	return json.Marshal(e)
}

func (e *Error) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, e)
}

func (e *Error) Error() string {
	return fmt.Sprintf("protocol error %d: %s", e.Code, e.Message)
}
