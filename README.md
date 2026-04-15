# vshell - Secure Remote Control Software

vshell is a secure, high-performance remote control solution for command-line remote management, file transfer, and session management across POSIX and Windows platforms.

## Features

### Core Features
- **Remote Terminal**: Full PTY support on Unix, ConPTY on Windows 10+
- **Session Management**: UUID-based sessions with activity tracking
- **Protocol**: Custom binary protocol v1 with channel multiplexing
- **Security**: TLS 1.3 with optional mutual TLS (mTLS) authentication
- **File Transfer**: Bidirectional file transfer with resume support and SHA256 checksums
- **Structured Logging**: JSON-based structured logging with configurable levels

### Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                         vshell                               │
├─────────────────────────────────────────────────────────────┤
│  Client (cmd/vshell-client)      Server (cmd/vshell-server) │
│         │                                    │              │
│         ▼                                    ▼              │
│  ┌──────────────┐                     ┌──────────────┐      │
│  │    TLS       │◄───────────────────►│    TLS       │      │
│  └──────────────┘                     └──────────────┘      │
│  ┌──────────────┐                     ┌──────────────┐      │
│  │  Protocol    │◄───────────────────►│  Protocol    │      │
│  └──────────────┘                     └──────────────┘      │
│  ┌──────────────┐                     ┌──────────────┐      │
│  │   Shell      │◄───────────────────►│   Shell      │      │
│  │   PTY        │                     │   PTY        │      │
│  └──────────────┘                     └──────────────┘      │
│  ┌──────────────┐                     ┌──────────────┐      │
│  │  File TX     │◄───────────────────►│  File TX     │      │
│  └──────────────┘                     └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Build

```bash
# Requirements
# - Go 1.21+
# - Unix: creack/pty for PTY support
# - Windows: ConPTY (built into Windows 10+)

# Build server
go build -o vshell-server ./cmd/vshell-server

# Build client
go build -o vshell-client ./cmd/vshell-client
```

### Generate TLS Certificates

```bash
# Generate CA key and certificate
openssl req -x509 -newkey rsa:4096 -keyout ca.key -out ca.crt -days 365 -nodes -subj "/CN=vshell CA"

# Generate server certificate
openssl req -newkey rsa:4096 -keyout server.key -out server.csr -nodes -subj "/CN=vshell-server"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365

# Generate client certificate
openssl req -newkey rsa:4096 -keyout client.key -out client.csr -nodes -subj "/CN=vshell-client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365
```

### Run Server

```bash
./vshell-server \
    -a 0.0.0.0:2222 \
    -c server.crt \
    -k server.key \
    --ca ca.crt \
    --mtls \
    -l DEBUG
```

### Run Client

```bash
./vshell-client \
    -a localhost:2222 \
    -c client.crt \
    -k client.key \
    --ca ca.crt \
    -l DEBUG
```

## Project Structure

```
vshellProject/
├── cmd/
│   ├── vshell-server/    # Server executable
│   └── vshell-client/    # Client executable
├── pkg/
│   ├── protocol/         # Wire protocol (frames, multiplexing, handshake)
│   ├── transport/        # TLS transport layer
│   ├── shell/            # PTY/ConPTY shell abstractions
│   ├── session/          # Session management
│   ├── file/             # File transfer
│   ├── logging/          # Structured logging
│   └── auth/             # Authentication (stub)
├── tests/
│   ├── integration/      # Integration tests
│   └── benchmark/        # Performance benchmarks
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
└── README.md             # This file
```

## Protocol

### Frame Format
```
[Version:1][Channel:1][Type:1][Length:4][Payload:N][CRC32:4]
```

- **Version**: Protocol version (1 byte)
- **Channel**: Channel ID (1 byte)
  - 00: Control
  - 01: Shell
  - 02: File
- **Type**: Message type (1 byte)
- **Length**: Payload length in bytes (4 bytes, big-endian)
- **Payload**: Message payload (N bytes)
- **CRC32**: IEEE CRC32 checksum (4 bytes)

### Handshake

1. Client sends `Hello` frame on Channel 0
2. Server responds with `Ok` or `Error` frame
3. Session is established

## Configuration

### Server Options

| Option | Description | Default |
|--------|-------------|---------|
| `-a` | Listen address | `0.0.0.0:22` |
| `-c` | Certificate file (required) | - |
| `-k` | Key file (required) | - |
| `--ca` | CA certificate file | - |
| `--mtls` | Enable mutual TLS | `false` |
| `-s` | Max concurrent sessions | 100 |
| `-l` | Log level | INFO |

### Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `-a` | Server address | `localhost:22` |
| `-c` | Client certificate file | - |
| `-k` | Client key file | - |
| `--ca` | CA certificate file | - |
| `-i` | Skip server certificate verification | `false` |
| `-l` | Log level | INFO |

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test -v ./tests/integration/...

# Run benchmarks
go test -bench=. ./tests/benchmark/...

# Run with race detector
go test -race ./...
```

## License

MIT License - See LICENSE file for details.

## Roadmap

- [ ] WebSocket transport support
- [ ] QUIC transport support
- [ ] Connection pooling
- [ ] Command forwarding modes
- [ ] SCP/SFTP compatibility layer
