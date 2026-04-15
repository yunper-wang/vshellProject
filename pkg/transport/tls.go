package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"
)

// TLSTransport implements Transport with TLS support
type TLSTransport struct {
	listener net.Listener
	config   *tls.Config
	addr     net.Addr
}

// NewTLSTransport creates a new TLS transport
func NewTLSTransport(address string, config *TLSConfig) (*TLSTransport, error) {
	tlsConfig, err := BuildServerTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to start TLS listener: %w", err)
	}

	return &TLSTransport{
		listener: listener,
		config:   tlsConfig,
		addr:     listener.Addr(),
	}, nil
}

// Accept waits for incoming TLS connections
func (t *TLSTransport) Accept() (net.Conn, error) {
	return t.listener.Accept()
}

// Close closes the transport listener
func (t *TLSTransport) Close() error {
	return t.listener.Close()
}

// Addr returns the local address
func (t *TLSTransport) Addr() net.Addr {
	return t.addr
}

// TLSClientTransport implements ClientTransport with TLS support
type TLSClientTransport struct {
	config *tls.Config
}

// NewTLSClientTransport creates a new TLS client transport
func NewTLSClientTransport(config *TLSConfig) (*TLSClientTransport, error) {
	tlsConfig, err := BuildClientTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build client TLS config: %w", err)
	}

	return &TLSClientTransport{
		config: tlsConfig,
	}, nil
}

// Dial connects to the server with TLS
func (t *TLSClientTransport) Dial(address string, timeout time.Duration) (net.Conn, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: timeout,
	}, "tcp", address, t.config)
	if err != nil {
		return nil, fmt.Errorf("TLS dial failed: %w", err)
	}

	// Verify handshake completed
	if err := conn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Verify peer certificate
	state := conn.ConnectionState()
	if !state.HandshakeComplete {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake not complete")
	}

	return conn, nil
}

// BuildServerTLSConfig builds TLS config for server
func BuildServerTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("TLS config required")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
	}

	// Load certificate
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load server certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		return nil, fmt.Errorf("server certificate and key files required")
	}

	// mTLS configuration
	if cfg.ClientAuth {
		if cfg.CAFile == "" {
			return nil, fmt.Errorf("CA file required for mTLS")
		}

		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

// BuildClientTLSConfig builds TLS config for client
func BuildClientTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("TLS config required")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
		InsecureSkipVerify: cfg.Insecure,
	}

	// Load client certificate for mTLS
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for server verification
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// GenerateSelfSignedCert generates self-signed certificate for testing (placeholder)
func GenerateSelfSignedCert() (cert, key []byte, err error) {
	// This is a placeholder. In production, use proper CA or certificate generation tools
	return nil, nil, fmt.Errorf("GenerateSelfSignedCert not implemented - use external CA")
}
