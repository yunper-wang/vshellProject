package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"vshellProject/pkg/logging"
	"vshellProject/pkg/protocol"
	"vshellProject/pkg/session"
	"vshellProject/pkg/transport"
)

var (
	serverAddr     string
	serverCertFile string
	serverKeyFile  string
	serverCAFile   string
	mtlsEnabled    bool
	maxSessions    int
	logLevel       string
)

func init() {
	rootCmd.Flags().StringVarP(&serverAddr, "address", "a", "0.0.0.0:22", "Server listen address")
	rootCmd.Flags().StringVarP(&serverCertFile, "cert", "c", "", "TLS certificate file")
	rootCmd.Flags().StringVarP(&serverKeyFile, "key", "k", "", "TLS key file")
	rootCmd.Flags().StringVarP(&serverCAFile, "ca", "", "", "CA certificate file for mTLS")
	rootCmd.Flags().BoolVarP(&mtlsEnabled, "mtls", "m", false, "Enable mutual TLS")
	rootCmd.Flags().IntVarP(&maxSessions, "max-sessions", "s", 100, "Maximum concurrent sessions")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
}

var rootCmd = &cobra.Command{
	Use:   "vshell-server",
	Short: "vshell remote control server",
	Long:  `vshell server - secure remote shell and file transfer server`,
	Run:   runServer,
}

func runServer(cmd *cobra.Command, args []string) {
	// Setup logging
	logger := logging.New(logging.ParseLevel(logLevel), os.Stdout)
	logging.SetDefault(logger)

	logger.Infof("Starting vshell server on %s", serverAddr)

	// Create TLS config
	tlsConfig := &transport.TLSConfig{
		CertFile:   serverCertFile,
		KeyFile:    serverKeyFile,
		CAFile:     serverCAFile,
		ClientAuth: mtlsEnabled,
	}

	// Create transport
	svr, err := transport.NewTLSTransport(serverAddr, tlsConfig)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}
	defer svr.Close()

	// Create session manager
	mgr := session.NewManager(maxSessions, 0)

	logger.Infof("Server ready, accepting connections (max sessions: %d)", maxSessions)

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutting down server...")
		svr.Close()
		os.Exit(0)
	}()

	// Accept loop
	for {
		conn, err := svr.Accept()
		if err != nil {
			logger.Errorf("Accept error: %v", err)
			continue
		}

		logger.Infof("New connection from %s", conn.RemoteAddr())

		go handleConnection(conn, mgr, logger)
	}
}

func handleConnection(rawConn net.Conn, mgr *session.Manager, logger *logging.Logger) {
	// Wrap with protocol
	conn := protocol.NewConn(rawConn)
	defer protocol.CloseConnection(conn)

	// Handshake
	clientHello, okResp, err := protocol.ServerHandshake(conn, protocol.DefaultServerInfo())
	if err != nil {
		logger.Errorf("Handshake failed: %v", err)
		return
	}

	logger.Infof("Client handshake: version=%d, features=%v", clientHello.Version, clientHello.Features)

	// Send OK response
	if err := protocol.SendOk(conn, okResp); err != nil {
		logger.Errorf("Failed to send OK: %v", err)
		return
	}

	// Create session
	sess, err := mgr.Create()
	if err != nil {
		logger.Errorf("Failed to create session: %v", err)
		return
	}
	defer mgr.Destroy(sess.ID)

	logger.Infof("Session created: %s", sess.ID)

	// Process frames
	for {
		frame, err := conn.ReadFrame()
		if err != nil {
			if err != io.EOF {
				logger.Errorf("Read error: %v", err)
			}
			break
		}

		// Update activity
		mgr.UpdateActivity(sess.ID)

		// Handle frame by channel
		switch frame.Channel {
		case protocol.ChannelControl:
			handleControlFrame(conn, sess, frame, logger)
		case protocol.ChannelShell:
			handleShellFrame(conn, sess, frame, logger)
		case protocol.ChannelFile:
			handleFileFrame(conn, sess, frame, logger)
		default:
			logger.Warnf("Unknown channel: %d", frame.Channel)
		}
	}
}

func handleControlFrame(conn *protocol.Conn, sess *session.Session, frame *protocol.Frame, logger *logging.Logger) {
	switch frame.Type {
	case protocol.TypeHeartbeat:
		// Respond with heartbeat
		hb := protocol.NewHeartbeat()
		payload, _ := hb.MarshalBinary()
		conn.WriteFrame(protocol.NewFrame(protocol.ChannelControl, protocol.TypeHeartbeat, payload))
	case protocol.TypeDisconnect:
		logger.Info("Client requested disconnect")
		protocol.CloseConnection(conn)
	default:
		logger.Warnf("Unknown control message: %d", frame.Type)
	}
}

func handleShellFrame(conn *protocol.Conn, sess *session.Session, frame *protocol.Frame, logger *logging.Logger) {
	// Placeholder for shell handling
	logger.Debugf("Shell frame received: type=%d, len=%d", frame.Type, len(frame.Payload))
}

func handleFileFrame(conn *protocol.Conn, sess *session.Session, frame *protocol.Frame, logger *logging.Logger) {
	// Placeholder for file transfer handling
	logger.Debugf("File frame received: type=%d, len=%d", frame.Type, len(frame.Payload))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
