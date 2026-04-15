package integration

import (
	"bytes"
	"testing"

	"vshellProject/pkg/protocol"
)

func TestProtocolFrameEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		channel uint8
		msgType uint8
		payload []byte
	}{
		{"Control Hello", protocol.ChannelControl, protocol.TypeHello, []byte(`{"version":1,"features":["shell","file"]}`)},
		{"Shell Data", protocol.ChannelShell, protocol.TypeShellData, []byte("ls -la\n")},
		{"File Request", protocol.ChannelFile, protocol.TypeFileRequest, []byte(`{"path":"/tmp/test.txt"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create frame
			frame := protocol.NewFrame(tt.channel, tt.msgType, tt.payload)

			// Encode
			encoded, err := frame.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := protocol.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify
			if decoded.Version != frame.Version {
				t.Errorf("Version mismatch: got %d, want %d", decoded.Version, frame.Version)
			}
			if decoded.Channel != frame.Channel {
				t.Errorf("Channel mismatch: got %d, want %d", decoded.Channel, frame.Channel)
			}
			if decoded.Type != frame.Type {
				t.Errorf("Type mismatch: got %d, want %d", decoded.Type, frame.Type)
			}
			if !bytes.Equal(decoded.Payload, frame.Payload) {
				t.Errorf("Payload mismatch: got %q, want %q", decoded.Payload, frame.Payload)
			}
		})
	}
}

func TestProtocolFrameCRC(t *testing.T) {
	frame := protocol.NewFrame(protocol.ChannelControl, protocol.TypeHeartbeat, []byte("test"))
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Corrupt the data
	encoded[10] ^= 0xFF

	// Decode should fail
	_, err = protocol.Decode(encoded)
	if err != protocol.ErrCRCMismatch {
		t.Errorf("Expected CRC mismatch error, got: %v", err)
	}
}

func TestProtocolFrameTooLarge(t *testing.T) {
	largePayload := make([]byte, protocol.MaxPayloadSize+1)
	frame := protocol.NewFrame(protocol.ChannelShell, protocol.TypeShellData, largePayload)

	_, err := frame.Encode()
	if err != protocol.ErrPayloadTooBig {
		t.Errorf("Expected payload too big error, got: %v", err)
	}
}

func TestProtocolHandshakeMessages(t *testing.T) {
	// Test Hello message
	hello := protocol.NewHello(protocol.Version, []string{"shell", "pty", "file"}, &protocol.ClientInfo{
		OS:   "linux",
		Arch: "amd64",
	})

	payload, err := hello.MarshalBinary()
	if err != nil {
		t.Fatalf("Hello MarshalBinary failed: %v", err)
	}

	decodedHello := &protocol.Hello{}
	if err := decodedHello.UnmarshalBinary(payload); err != nil {
		t.Fatalf("Hello UnmarshalBinary failed: %v", err)
	}

	if decodedHello.Version != protocol.Version {
		t.Errorf("Hello version mismatch: got %d, want %d", decodedHello.Version, protocol.Version)
	}

	// Test Ok message
	ok := protocol.NewOk(protocol.Version, []string{"shell", "pty", "file"}, "session-123", &protocol.ServerInfo{
		OS:   "linux",
		Arch: "amd64",
	})

	payload, err = ok.MarshalBinary()
	if err != nil {
		t.Fatalf("Ok MarshalBinary failed: %v", err)
	}

	decodedOk := &protocol.Ok{}
	if err := decodedOk.UnmarshalBinary(payload); err != nil {
		t.Fatalf("Ok UnmarshalBinary failed: %v", err)
	}

	if decodedOk.SessionID != "session-123" {
		t.Errorf("Ok session ID mismatch: got %s, want %s", decodedOk.SessionID, "session-123")
	}
}
