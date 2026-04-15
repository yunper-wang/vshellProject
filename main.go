// vshell - Secure Remote Control Software
//
// vshell is a secure, high-performance remote control solution supporting:
// - Remote terminal/shell access with PTY (Unix) and ConPTY (Windows)
// - Bidirectional file transfer with resume support
// - Session management and resilience
// - TLS 1.3 with optional mTLS authentication
//
// Usage:
//   # Run server
//   go run ./cmd/vshell-server -a localhost:2222 -c server.crt -k server.key
//
//   # Run client
//   go run ./cmd/vshell-client -a localhost:2222 -c client.crt -k client.key --ca ca.crt

package main

import (
	"fmt"
)

// Entry point placeholder - use cmd/vshell-server or cmd/vshell-client
func main() {
	fmt.Println("vshell - Secure Remote Control Software")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  vshell-server - Run the server")
	fmt.Println("  vshell-client - Run the client")
	fmt.Println("")
	fmt.Println("See cmd/ directory for commands")
}
