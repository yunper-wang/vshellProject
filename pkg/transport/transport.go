package transport

import (
	"net"
	"time"
)

// Transport defines the transport layer interface for vshell
// Implementations: TLS, WebSocket, and future QUIC
type Transport interface {
	// Accept waits for incoming connections
	Accept() (net.Conn, error)

	// Close closes the transport listener
	Close() error

	// Addr returns the local address
	Addr() net.Addr
}

// ClientTransport defines the client-side transport interface
type ClientTransport interface {
	// Dial connects to the server
	Dial(address string, timeout time.Duration) (net.Conn, error)
}

// Config holds common transport configuration
type Config struct {
	Address        string
	TLSConfig      *TLSConfig
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxConnections int
}

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	Insecure   bool
	ClientAuth bool
}
