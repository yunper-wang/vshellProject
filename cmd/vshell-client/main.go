package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"vshellProject/pkg/logging"
	"vshellProject/pkg/protocol"
	"vshellProject/pkg/transport"
)

var (
	clientAddr     string
	clientCertFile string
	clientKeyFile  string
	clientCAFile   string
	insecure       bool
	clientLogLevel string
)

func init() {
	rootCmd.Flags().StringVarP(&clientAddr, "address", "a", "localhost:22", "Server address")
	rootCmd.Flags().StringVarP(&clientCertFile, "cert", "c", "", "Client certificate file (for mTLS)")
	rootCmd.Flags().StringVarP(&clientKeyFile, "key", "k", "", "Client key file (for mTLS)")
	rootCmd.Flags().StringVarP(&clientCAFile, "ca", "", "", "CA certificate file")
	rootCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip server certificate verification")
	rootCmd.Flags().StringVarP(&clientLogLevel, "log-level", "l", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
}

var rootCmd = &cobra.Command{
	Use:   "vshell-client",
	Short: "vshell remote control client",
	Long:  `vshell client - secure remote shell and file transfer client`,
	Run:   runClient,
}

func runClient(cmd *cobra.Command, args []string) {
	// Setup logging
	logger := logging.New(logging.ParseLevel(clientLogLevel), os.Stdout)
	logging.SetDefault(logger)

	logger.Infof("Connecting to %s", clientAddr)

	// Create TLS config
	tlsConfig := &transport.TLSConfig{
		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,
		CAFile:   clientCAFile,
		Insecure: insecure,
	}

	// Create client transport
	client, err := transport.NewTLSClientTransport(tlsConfig)
	if err != nil {
		logger.Fatalf("Failed to create client: %v", err)
	}

	// Connect to server
	conn, err := client.Dial(clientAddr, 10*time.Second)
	if err != nil {
		logger.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	logger.Info("Connected to server")

	// Wrap with protocol
	protoConn := protocol.NewConn(conn)

	// Handshake
	okResp, err := protocol.ClientHandshake(protoConn, protocol.DefaultClientInfo())
	if err != nil {
		logger.Fatalf("Handshake failed: %v", err)
	}

	logger.Infof("Handshake successful: session=%s, features=%v", okResp.SessionID, okResp.Features)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start heartbeat
	go heartbeatLoop(protoConn, logger)

	// Start input loop
	go inputLoop(protoConn, logger)

	// Receive loop
	go receiveLoop(protoConn, logger)

	// Wait for signal
	<-sigChan
	logger.Info("Disconnecting...")
}

func heartbeatLoop(conn *protocol.Conn, logger *logging.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		hb := protocol.NewHeartbeat()
		payload, err := hb.MarshalBinary()
		if err != nil {
			logger.Errorf("Failed to marshal heartbeat: %v", err)
			continue
		}

		if err := conn.WriteFrame(protocol.NewFrame(protocol.ChannelControl, protocol.TypeHeartbeat, payload)); err != nil {
			logger.Errorf("Failed to send heartbeat: %v", err)
			return
		}
	}
}

func inputLoop(conn *protocol.Conn, logger *logging.Logger) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				logger.Errorf("Input error: %v", err)
			}
			return
		}

		// Send to shell channel
		frame := protocol.NewFrame(protocol.ChannelShell, protocol.TypeShellData, []byte(line))
		if err := conn.WriteFrame(frame); err != nil {
			logger.Errorf("Failed to send: %v", err)
			return
		}
	}
}

func receiveLoop(conn *protocol.Conn, logger *logging.Logger) {
	for {
		frame, err := conn.ReadFrame()
		if err != nil {
			if err != io.EOF {
				logger.Errorf("Receive error: %v", err)
			}
			return
		}

		switch frame.Channel {
		case protocol.ChannelShell:
			// Write to stdout
			os.Stdout.Write(frame.Payload)
		case protocol.ChannelControl:
			handleControlMessage(frame, logger)
		case protocol.ChannelFile:
			handleFileMessage(frame, logger)
		}
	}
}

func handleControlMessage(frame *protocol.Frame, logger *logging.Logger) {
	switch frame.Type {
	case protocol.TypeHeartbeat:
		// Heartbeat received, ignore
	case protocol.TypeError:
		var e protocol.Error
		if err := e.UnmarshalBinary(frame.Payload); err == nil {
			logger.Errorf("Server error: %s", e.Message)
		}
	default:
		logger.Debugf("Control message: type=%d", frame.Type)
	}
}

func handleFileMessage(frame *protocol.Frame, logger *logging.Logger) {
	logger.Debugf("File message: type=%d, len=%d", frame.Type, len(frame.Payload))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
