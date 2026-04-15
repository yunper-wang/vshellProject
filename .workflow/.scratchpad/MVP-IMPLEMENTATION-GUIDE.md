# vShell MVP Implementation Guide
## Pragmatic Approach B (6-Week Timeline)

---

## Executive Summary

**Approach**: TLS over TCP + WebSocket fallback, custom binary protocol with channel multiplexing
**Timeline**: 6 weeks (240 hours)
**Team**: 2-3 engineers (1 backend, 1 cross-platform, 0.5 QA)
**Tech Stack**: Go 1.26, crypto/tls, gorilla/websocket, spf13/cobra, creack/pty

**Core Deliverables**:
- Remote shell execution (Unix + Windows 10+)
- Bidirectional file transfer with resume
- Session persistence and reconnection
- Mutual TLS authentication
- Cross-platform CLI client

---

## 1. Six-Week MVP Roadmap

### Week 1: Protocol Design & Transport Layer (40h)

**Objective**: Establish reliable encrypted transport and binary framing protocol

#### Day 1-2: Protocol Specification (16h)
```
Tasks:
├─ Define Protocol v1 binary frame format (8h)
│  ├─ Header: Version(1) + Channel(1) + Type(1) + Length(4) = 7 bytes
│  ├─ Payload: Max 4MB per frame
│  └─ Trailer: CRC32(4) for integrity
├─ Document channel allocation strategy (2h)
│  ├─ Channel 0: Control (session management, heartbeat)
│  ├─ Channel 1: Shell PTY data
│  ├─ Channel 2: File transfer
│  └─ Channel 3-255: Reserved for future use
├─ Design version negotiation handshake (4h)
│  ├─ Client: HELLO{version=1, features=[shell,file]}
│  ├─ Server: OK{version=1, features=[shell,file], session_id=uuid}
│  └─ Backward compatibility: if version mismatch, use max common version
└─ Create protocol specification document (2h)
```

**Protocol v1 Frame Format (Byte-Level)**:
```
+--------+--------+--------+----------+----------+----------+
| Ver(1) | Chan(1)| Type(1)| Len(4)   | Payload  | CRC32(4) |
+--------+--------+--------+----------+----------+----------+
| 0x01   | 0x00   | 0x01   | 0x000010 | <N bytes>| 0xA5B3.. |
+--------+--------+--------+----------+----------+----------+

Message Types:
  0x01 = DATA        (payload contains application data)
  0x02 = ACK         (acknowledgment, payload = sequence number)
  0x03 = NACK        (negative acknowledgment, payload = error code)
  0x04 = HEARTBEAT   (payload = timestamp)
  0x05 = CLOSE       (payload = close reason)
  0x06 = RESET       (reset channel state)
```

#### Day 3-4: TLS Transport Implementation (16h)
```
Tasks:
├─ Implement TLS 1.3 server listener (6h)
│  ├─ Load certificates from file or generate self-signed
│  ├─ Configure cipher suites: TLS_AES_256_GCM_SHA384, TLS_CHACHA20_POLY1305_SHA256
│  └─ Set MinVersion = TLS 1.3
├─ Implement TLS client dialer (4h)
│  ├─ Verify server certificate (optional skip for dev)
│  └─ Configure client certificates for mTLS
├─ Add connection pooling and reuse (4h)
│  ├─ Keep-alive with 30s idle timeout
│  └─ Max 10 connections per client
└─ Write unit tests for transport layer (2h)
```

**Code Structure**:
```go
// pkg/transport/tls.go
type TLSListener struct {
    config    *tls.Config
    listeners map[string]net.Listener // key: addr
}

func (l *TLSListener) Accept() (net.Conn, error)
func (l *TLSListener) handshake(conn net.Conn) (*Session, error)

// pkg/transport/tls_client.go
type TLSClient struct {
    config     *tls.Config
    pool       *ConnectionPool
    timeout    time.Duration
}

func (c *TLSClient) Dial(addr string) (net.Conn, error)
func (c *TLSClient) DialWithCert(addr string, cert tls.Certificate) (net.Conn, error)
```

#### Day 5: WebSocket Fallback (8h)
```
Tasks:
├─ Implement WebSocket server upgrade (4h)
│  ├─ Use gorilla/websocket Upgrader
│  └─ Wrap WebSocket as net.Conn interface
└─ Implement WebSocket client with auto-fallback (4h)
   ├─ Try TLS first, fallback to WebSocket on timeout
   └─ WebSocket URL: wss://host:port/ws
```

**WebSocket Wrapper**:
```go
// pkg/transport/websocket.go
type WSConn struct {
    *websocket.Conn
    r io.Reader
}

func (c *WSConn) Read(b []byte) (int, error)
func (c *WSConn) Write(b []byte) (int, error)
func (c *WSConn) Close() error
func (c *WSConn) LocalAddr() net.Addr
func (c *WSConn) RemoteAddr() net.Addr
```

**Risks & Mitigation**:
- Risk: Protocol design flaw discovered late → Mitigation: Week 1 review with team, freeze protocol on Day 2
- Risk: TLS configuration incompatibilities → Mitigation: Test with OpenSSL 3.0 and BoringSSL

**Deliverables**:
- [ ] Protocol v1 specification document
- [ ] TLS server/client with mTLS support
- [ ] WebSocket fallback implementation
- [ ] Unit tests with >80% coverage

---

### Week 2: Shell Execution on Unix (40h)

**Objective**: Implement PTY-based shell execution with proper I/O handling

#### Day 1-2: PTY Integration (16h)
```
Tasks:
├─ Integrate creack/pty for Unix (8h)
│  ├─ pty.Start(cmd) with window size
│  ├─ Handle SIGWINCH for resize
│  └─ Set terminal to raw mode with golang.org/x/term
├─ Shell command execution engine (6h)
│  ├─ Support /bin/sh, /bin/bash, /usr/bin/zsh
│  ├─ Environment variable inheritance
│  └─ Working directory configuration
└─ Test with interactive commands (2h)
   ├─ vim, top, htop
   └─ Password prompts (sudo)
```

**PTY Manager**:
```go
// pkg/shell/unix.go
type UnixShell struct {
    cmd       *exec.Cmd
    pty       *os.File
    winsize   *pty.Winsize
    exitCode  int
}

func NewUnixShell(cmd string, args []string, opts ShellOptions) (*UnixShell, error)
func (s *UnixShell) Start() error
func (s *UnixShell) Read(p []byte) (n int, err error)
func (s *UnixShell) Write(p []byte) (n int, err error)
func (s *UnixShell) Resize(cols, rows uint16) error
func (s *UnixShell) Close() error
func (s *UnixShell) Wait() (*ExitStatus, error)
```

#### Day 3-4: Shell Channel Multiplexing (16h)
```
Tasks:
├─ Implement channel 1 handler (8h)
│  ├─ Map shell session ID to PTY instance
│  ├─ Bidirectional data streaming (shell <-> channel)
│  └─ Flow control: pause reading if buffer > 1MB
├─ Session management (6h)
│  ├─ Create/destroy shell sessions
│  ├─ Track active sessions per connection
│  └─ Cleanup on connection close
└─ Integration tests (2h)
   ├─ Multiple concurrent shells
   └─ Session lifecycle
```

**Session Manager**:
```go
// pkg/session/manager.go
type SessionManager struct {
    sessions map[uint64]*ShellSession
    mu       sync.RWMutex
    nextID   uint64
}

func (m *SessionManager) Create(conn net.Conn, opts ShellOptions) (*ShellSession, error)
func (m *SessionManager) Get(id uint64) (*ShellSession, error)
func (m *SessionManager) Destroy(id uint64) error
func (m *SessionManager) CloseAll() error

type ShellSession struct {
    ID        uint64
    Shell     Shell
    Conn      net.Conn
    CreatedAt time.Time
    LastIO    time.Time
}
```

#### Day 5: Control Channel & Heartbeat (8h)
```
Tasks:
├─ Implement channel 0 handler (6h)
│  ├─ Heartbeat: client sends every 30s, server responds
│  ├─ Session creation requests
│  └─ Session termination requests
└─ Timeout handling (2h)
   ├─ Connection timeout: 60s no heartbeat = close
   └─ Graceful shutdown
```

**Control Protocol**:
```
Client -> Server:
  CREATE_SESSION {shell: "/bin/bash", rows: 24, cols: 80}
  DESTROY_SESSION {session_id: 123}
  HEARTBEAT {timestamp: 1234567890}

Server -> Client:
  SESSION_CREATED {session_id: 123, pty: "xterm-256color"}
  SESSION_DESTROYED {session_id: 123, exit_code: 0}
  HEARTBEAT_ACK {timestamp: 1234567890, latency: 5ms}
  ERROR {code: 404, message: "session not found"}
```

**Risks & Mitigation**:
- Risk: PTY permissions (container environments) → Mitigation: Document required capabilities, test in Docker
- Risk: Zombie processes → Mitigation: Proper cleanup in defer, reap children with sigchld

**Deliverables**:
- [ ] PTY-based shell execution on Unix
- [ ] Channel 1 shell data streaming
- [ ] Session management with cleanup
- [ ] Control channel with heartbeat

---

### Week 3: Windows PTY Support (40h)

**Objective**: Implement ConPTY-based shell execution for Windows 10+

#### Day 1-2: ConPTY Integration (20h)
```
Tasks:
├─ Research ConPTY Windows API (6h)
│  ├─ CreatePseudoConsole
│  ├─ ResizePseudoConsole
│  ├─ ClosePseudoConsole
│  └─ ConPTY pipe handling
├─ Implement ConPTY wrapper in Go (10h)
│  ├─ Call Windows DLLs via syscall
│  ├─ Create named pipes for input/output
│  ├─ Launch process with PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE
│  └─ Handle window resize events
└─ Test on Windows 10/11 (4h)
   ├─ PowerShell, cmd.exe
   └─ Interactive tools (less, vim Win32)
```

**ConPTY Implementation**:
```go
// pkg/shell/windows.go
type WindowsShell struct {
    pty        syscall.Handle
    process    syscall.Handle
    inputPipe  syscall.Handle
    outputPipe syscall.Handle
    size       pty.Winsize
}

func NewWindowsShell(cmd string, args []string, opts ShellOptions) (*WindowsShell, error) {
    // 1. Create named pipes for input/output
    // 2. Call CreatePseudoConsole
    // 3. Initialize STARTUPINFOEX with PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE
    // 4. CreateProcess with EXTENDED_STARTUPINFO_PRESENT
    // 5. Return shell instance
}

func (s *WindowsShell) Resize(cols, rows uint16) error {
    size := _COORD{X: int16(cols), Y: int16(rows)}
    return resizePseudoConsole(s.pty, size)
}

func (s *WindowsShell) Read(p []byte) (n int, err error) {
    // Read from outputPipe
}

func (s *WindowsShell) Write(p []byte) (n int, err error) {
    // Write to inputPipe
}
```

**Windows API Bindings**:
```go
// pkg/shell/windows_sys.go
type _COORD struct {
    X, Y int16
}

var (
    kernel32 = syscall.NewLazyDLL("kernel32.dll")

    procCreatePseudoConsole       = kernel32.NewProc("CreatePseudoConsole")
    procResizePseudoConsole       = kernel32.NewProc("ResizePseudoConsole")
    procClosePseudoConsole        = kernel32.NewProc("ClosePseudoConsole")
    procInitializeProcThreadAttributeList = kernel32.NewProc("InitializeProcThreadAttributeList")
    procUpdateProcThreadAttribute  = kernel32.NewProc("UpdateProcThreadAttribute")
)
```

#### Day 3: Windows Version Detection & Fallback (8h)
```
Tasks:
├─ Implement Windows version detection (4h)
│  ├─ Use RtlGetVersion or registry
│  ├─ ConPTY available: Windows 10 1809+ (build 17763+)
│  └─ Fallback: Windows 7/8, Windows Server 2012
├─ Fallback implementation: Named pipes (4h)
│  ├─ cmd.exe with stdin/stdout/stderr pipes
│  ├─ No PTY: limited interactive support
│  └─ Document limitations (no color, no resize)
```

**Platform Detection**:
```go
// pkg/shell/shell.go
func NewShell(cmd string, args []string, opts ShellOptions) (Shell, error) {
    if runtime.GOOS == "windows" {
        if supportsConPTY() {
            return NewWindowsShell(cmd, args, opts)
        }
        return NewWindowsPipeShell(cmd, args, opts) // Fallback
    }
    return NewUnixShell(cmd, args, opts)
}

func supportsConPTY() bool {
    major, minor, build := getWindowsVersion()
    return major >= 10 && build >= 17763
}

func getWindowsVersion() (major, minor, build uint32) {
    // RtlGetVersion or registry query
}
```

#### Day 4-5: Cross-Platform Abstraction & Testing (12h)
```
Tasks:
├─ Define Shell interface (4h)
│  ├─ Read(p []byte) (n int, err error)
│  ├─ Write(p []byte) (n int, err error)
│  ├─ Resize(cols, rows uint16) error
│  ├─ Close() error
│  └─ Wait() (*ExitStatus, error)
├─ Integration tests on both platforms (6h)
│  ├─ Unit tests for each shell implementation
│  ├─ Cross-platform test matrix
│  └─ CI: GitHub Actions on ubuntu-latest and windows-latest
└─ Document platform differences (2h)
```

**Shell Interface**:
```go
// pkg/shell/shell.go
type Shell interface {
    io.ReadWriteCloser
    Resize(cols, rows uint16) error
    Wait() (*ExitStatus, error)
    Pid() int
}

type ExitStatus struct {
    Code   int
    Signal os.Signal // Unix only
}
```

**Risks & Mitigation**:
- Risk: Windows 7/8 incompatibility → Mitigation: Document clear limitations, test on real hardware
- Risk: ConPTY API changes → Mitigation: Pin to Windows 10 1809+ API, test on multiple builds
- Risk: ConPTY memory leaks → Mitigation: Proper ClosePseudoConsole calls, verify with Process Explorer

**Deliverables**:
- [ ] ConPTY-based shell execution on Windows 10+
- [ ] Fallback implementation for older Windows
- [ ] Cross-platform Shell interface
- [ ] Platform-specific tests

---

### Week 4: File Transfer (40h)

**Objective**: Implement bidirectional file transfer with chunking and resume

#### Day 1-2: File Transfer Protocol (16h)
```
Tasks:
├─ Design file transfer messages (8h)
│  ├─ UPLOAD_REQUEST {path, size, mode, checksum}
│  ├─ UPLOAD_ACCEPT {offset, chunk_size}
│  ├─ UPLOAD_DATA {offset, data[]}
│  ├─ UPLOAD_ACK {offset}
│  ├─ UPLOAD_COMPLETE {final_checksum}
│  └─ ERROR {code, message}
├─ Chunking strategy (4h)
│  ├─ Chunk size: 64KB (configurable 4KB-4MB)
│  ├─ Sliding window: 8 chunks in-flight
│  └─ CRC32 per chunk
└─ Resume mechanism (4h)
   ├─ Client requests upload with expected checksum
   ├─ Server calculates existing file checksum
   ├─ If partial match: resume from offset
   └─ If mismatch: restart or ask user
```

**File Transfer Protocol**:
```
Upload Flow:
  Client -> Server:
    UPLOAD_REQUEST {
      path: "/tmp/file.tar.gz",
      size: 104857600,
      mode: 0644,
      checksum: "sha256:abc123..."
    }

  Server -> Client:
    UPLOAD_ACCEPT {
      offset: 0,           // 0 = new upload, >0 = resume
      chunk_size: 65536
    }

  Client -> Server:
    UPLOAD_DATA {offset: 0, data: <64KB>}
    UPLOAD_DATA {offset: 65536, data: <64KB>}
    ...

  Server -> Client:
    UPLOAD_ACK {offset: 65536}  // Ack every chunk
    UPLOAD_ACK {offset: 131072}

  Client -> Server:
    UPLOAD_COMPLETE {final_checksum: "sha256:def456..."}

  Server -> Client:
    UPLOAD_ACK {success: true, size: 104857600}

Download Flow:
  Client -> Server:
    DOWNLOAD_REQUEST {path: "/tmp/file.tar.gz", offset: 0}

  Server -> Client:
    DOWNLOAD_ACCEPT {size: 104857600, chunk_size: 65536, checksum: "sha256:abc..."}

  Server -> Client:
    DOWNLOAD_DATA {offset: 0, data: <64KB>}
    DOWNLOAD_DATA {offset: 65536, data: <64KB>}
    ...

  Client -> Server:
    DOWNLOAD_ACK {offset: 131072}  // Ack every chunk

  Server -> Client:
    DOWNLOAD_COMPLETE {final_checksum: "sha256:abc..."}

  Client -> Server:
    DOWNLOAD_ACK {success: true}
```

#### Day 3-4: File Transfer Implementation (16h)
```
Tasks:
├─ Implement file sender (client) (6h)
│  ├─ Read file in chunks
│  ├─ Calculate checksums (CRC32 + SHA256)
│  ├─ Send chunks with flow control
│  └─ Handle resume logic
├─ Implement file receiver (server) (6h)
│  ├─ Write chunks to temp file
│  ├─ Verify checksums
│  ├─ Move to final location on success
│  └─ Atomic rename on Unix, MoveFileEx on Windows
└─ Integration tests (4h)
   ├─ Small files (<1MB)
   ├─ Large files (>100MB)
   ├─ Resume scenarios
   └─ Error cases (disk full, permission denied)
```

**File Transfer Handler**:
```go
// pkg/file/transfer.go
type FileTransfer struct {
    conn       net.Conn
    chunkSize  int
    windowSize int
    tempDir    string
}

func (ft *FileTransfer) Upload(localPath, remotePath string, opts UploadOptions) error {
    // 1. Open local file
    // 2. Calculate SHA256 checksum
    // 3. Send UPLOAD_REQUEST
    // 4. Receive UPLOAD_ACCEPT
    // 5. Send chunks in sliding window
    // 6. Wait for ACKs
    // 7. Send UPLOAD_COMPLETE
}

func (ft *FileTransfer) Download(remotePath, localPath string, opts DownloadOptions) error {
    // 1. Send DOWNLOAD_REQUEST
    // 2. Receive DOWNLOAD_ACCEPT
    // 3. Receive chunks, write to temp file
    // 4. Send ACKs
    // 5. Verify final checksum
    // 6. Atomic rename temp -> final
}

type UploadOptions struct {
    ChunkSize    int
    Resume       bool
    ProgressChan chan<- Progress
}

type Progress struct {
    BytesTransferred int64
    TotalBytes       int64
    Percentage       float64
    Speed            int64 // bytes/sec
}
```

#### Day 5: Directory Listing & Management (8h)
```
Tasks:
├─ Implement directory listing (4h)
│  ├─ LIST_REQUEST {path}
│  ├─ LIST_RESPONSE {files: [{name, size, mode, modtime, is_dir}]}
│  └─ Support recursive listing with depth limit
└─ File management operations (4h)
   ├─ DELETE {path}
   ├─ RENAME {old_path, new_path}
   ├─ MKDIR {path, mode}
   └─ CHMOD {path, mode}  // Unix only
```

**Risks & Mitigation**:
- Risk: Large file memory usage → Mitigation: Stream chunks, never load entire file in memory
- Risk: Resume integrity → Mitigation: SHA256 per chunk + final checksum, verify before commit
- Risk: Path traversal attacks → Mitigation: Sanitize paths, use filepath.Clean, validate against root

**Deliverables**:
- [ ] Bidirectional file transfer with chunking
- [ ] Resume capability
- [ ] Directory listing and management
- [ ] Progress reporting

---

### Week 5: Resilience & Debugging (40h)

**Objective**: Add reconnection, logging, and observability

#### Day 1-2: Connection Resilience (16h)
```
Tasks:
├─ Implement automatic reconnection (8h)
│  ├─ Client detects connection loss
│  ├─ Exponential backoff: 1s, 2s, 4s, 8s, max 30s
│  ├─ Resume sessions if session_id still valid
│  └─ Re-establish shell sessions with PTY state
├─ Connection state machine (4h)
│  ├─ States: DISCONNECTED, CONNECTING, CONNECTED, RECONNECTING
│  └─ State transitions with proper cleanup
└─ Test reconnection scenarios (4h)
   ├─ Network cable unplug
   ├─ Server restart
   └─ Sleep/wake cycles
```

**Reconnection Logic**:
```go
// pkg/client/reconnect.go
type ReconnectingClient struct {
    client       *Client
    state        ConnectionState
    sessionID    string
    backoff      BackoffStrategy
    sessions     map[uint64]SessionState // Shell sessions to resume
}

func (rc *ReconnectingClient) Connect() error {
    for {
        err := rc.client.Connect(rc.sessionID)
        if err == nil {
            rc.onConnected()
            return nil
        }

        if !rc.shouldRetry(err) {
            return err
        }

        time.Sleep(rc.backoff.Next())
    }
}

func (rc *ReconnectingClient) onConnected() {
    // Re-establish shell sessions
    for _, state := range rc.sessions {
        rc.client.CreateShell(state.Cmd, state.Args, state.Size)
    }
}
```

#### Day 3: Heartbeat & Keepalive (8h)
```
Tasks:
├─ Implement heartbeat mechanism (6h)
│  ├─ Client sends HEARTBEAT every 30s
│  ├─ Server responds with HEARTBEAT_ACK
│  ├─ Measure round-trip latency
│  └─ Close connection if no response after 3 heartbeats
└─ Configure keepalive parameters (2h)
   ├─ TCP KeepAlive: 60s
   ├─ TLS keepalive: application-level heartbeat
   └─ WebSocket ping/pong
```

**Heartbeat Handler**:
```go
// pkg/session/heartbeat.go
type HeartbeatManager struct {
    interval    time.Duration
    timeout     time.Duration
    lastBeat    time.Time
    latency     time.Duration
    missedBeats int
}

func (h *HeartbeatManager) Send(conn net.Conn) error {
    msg := HeartbeatMessage{Timestamp: time.Now().UnixNano()}
    _, err := conn.Write(Encode(msg))
    return err
}

func (h *HeartbeatManager) Receive(msg HeartbeatMessage) {
    h.lastBeat = time.Now()
    h.latency = time.Since(time.Unix(0, msg.Timestamp))
    h.missedBeats = 0
}

func (h *HeartbeatManager) Check() error {
    if time.Since(h.lastBeat) > h.interval*3 {
        return ErrConnectionLost
    }
    return nil
}
```

#### Day 4-5: Logging & Debugging (16h)
```
Tasks:
├─ Structured logging with zerolog (8h)
│  ├─ Log levels: TRACE, DEBUG, INFO, WARN, ERROR
│  ├─ Context: connection_id, session_id, channel
│  ├─ Log rotation: size-based (10MB) + time-based (daily)
│  └─ Sensitive data redaction (passwords, keys)
├─ Debug mode CLI flag (4h)
│  ├─ --debug: enable DEBUG level
│  ├─ --trace: enable TRACE level
│  └─ --log-file: custom log path
└─ Metrics collection (4h)
   ├─ Connection count
   ├─ Active sessions
   ├─ Bytes transferred
   └─ Latency percentiles
```

**Logging Structure**:
```go
// pkg/logging/logger.go
import "github.com/rs/zerolog/log"

type Logger struct {
    zerolog.Logger
    connID    string
    sessionID string
}

func NewLogger(connID, sessionID string) *Logger {
    return &Logger{
        Logger:    log.With().Str("conn", connID).Str("session", sessionID).Logger(),
        connID:    connID,
        sessionID: sessionID,
    }
}

func (l *Logger) ShellInput(data []byte) {
    l.Debug().Hex("data", data).Msg("shell input")
}

func (l *Logger) ShellOutput(data []byte) {
    l.Debug().Hex("data", data).Msg("shell output")
}

func (l *Logger) FileTransfer(op, path string, progress int64) {
    l.Info().Str("op", op).Str("path", path).Int64("progress", progress).Msg("file transfer")
}

func (l *Logger) Error(err error, msg string) {
    l.Logger.Error().Err(err).Msg(msg)
}
```

**Risks & Mitigation**:
- Risk: Reconnection fails in production → Mitigation: Extensive testing with network simulators
- Risk: Logging performance overhead → Mitigation: Use zerolog (zero-allocation), async writes
- Risk: Sensitive data leaks → Mitigation: Redaction middleware, security review

**Deliverables**:
- [ ] Automatic reconnection with session resume
- [ ] Heartbeat with latency measurement
- [ ] Structured logging with rotation
- [ ] Debug mode CLI flags

---

### Week 6: Testing, Documentation, & Polish (40h)

**Objective**: Comprehensive testing, documentation, and CI/CD setup

#### Day 1-2: Integration Testing (16h)
```
Tasks:
├─ End-to-end test suite (8h)
│  ├─ Test: Connect, execute shell, disconnect
│  ├─ Test: Upload file, download file, verify checksum
│  ├─ Test: Multiple concurrent sessions
│  └─ Test: Reconnection scenarios
├─ Performance testing (4h)
│  ├─ Throughput: file transfer speed
│  ├─ Latency: shell responsiveness
│  └─ Concurrency: 10, 50, 100 sessions
└─ Stress testing (4h)
   ├─ Long-running sessions (24h+)
   ├─ High throughput (100MB/s)
   └─ Memory leak detection (pprof)
```

**Integration Test Example**:
```go
// tests/integration/shell_test.go
func TestShellSession(t *testing.T) {
    // 1. Start server
    server := NewTestServer(t)
    defer server.Close()

    // 2. Connect client
    client := NewTestClient(t, server.Addr())
    defer client.Close()

    // 3. Create shell session
    session, err := client.CreateShell("/bin/bash", nil, 24, 80)
    require.NoError(t, err)

    // 4. Execute command
    session.Write([]byte("echo hello\n"))
    output := session.ReadLine()
    assert.Contains(t, output, "hello")

    // 5. Resize terminal
    err = session.Resize(40, 120)
    require.NoError(t, err)

    // 6. Exit
    session.Write([]byte("exit\n"))
    exitStatus := session.Wait()
    assert.Equal(t, 0, exitStatus.Code)
}
```

#### Day 3: Documentation (12h)
```
Tasks:
├─ README.md (4h)
│  ├─ Project overview
│  ├─ Quick start guide
│  ├─ Installation instructions
│  └─ Basic usage examples
├─ ARCHITECTURE.md (4h)
│  ├─ System design
│  ├─ Protocol specification
│  ├─ Module breakdown
│  └─ Diagrams (Mermaid)
└─ CONTRIBUTING.md (4h)
   ├─ Development setup
   ├─ Code style guide
   ├─ Testing requirements
   └─ Pull request process
```

**README Structure**:
```markdown
# vShell - Cross-Platform Remote Shell

## Features
- Remote shell execution (Unix + Windows)
- Bidirectional file transfer with resume
- mTLS authentication
- Automatic reconnection

## Quick Start
### Server
\`\`\`bash
vshell-server --cert server.crt --key server.key --listen :8443
\`\`\`

### Client
\`\`\`bash
vshell-client --connect server:8443 --cert client.crt --key client.key
\`\`\`

## Installation
### From Source
\`\`\`bash
go install github.com/yourname/vshell/cmd/vshell-server@latest
go install github.com/yourname/vshell/cmd/vshell-client@latest
\`\`\`

### Pre-built Binaries
Download from [Releases](https://github.com/yourname/vshell/releases)

## Documentation
- [Architecture](./docs/ARCHITECTURE.md)
- [Protocol Specification](./docs/PROTOCOL.md)
- [API Reference](./docs/API.md)
```

#### Day 4: CI/CD Pipeline (8h)
```
Tasks:
├─ GitHub Actions workflow (4h)
│  ├─ Build: linux-amd64, linux-arm64, windows-amd64, darwin-amd64, darwin-arm64
│  ├─ Test: unit tests, integration tests
│  ├─ Lint: golangci-lint
│  └─ Release: goreleaser
└─ Docker images (4h)
   ├─ Multi-stage Dockerfile
   ├─ Base: alpine:3.18 (5MB)
   └─ Tags: latest, v1.0.0
```

**GitHub Actions**:
```yaml
# .github/workflows/ci.yml
name: CI

on: [push, pull_request]

jobs:
  build:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.26'
      - run: go build ./...
      - run: go test -race -coverprofile=coverage.txt -covermode=atomic ./...
      - uses: codecov/codecov-action@v3

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: golangci/golangci-lint-action@v3
        with:
          version: latest
```

#### Day 5: Final Polish (4h)
```
Tasks:
├─ Security audit (2h)
│  ├─ Review TLS configuration
│  ├─ Check for path traversal
│  └─ Verify credential handling
└─ Performance profiling (2h)
   ├─ CPU profile: go tool pprof
   ├─ Memory profile: go tool pprof -alloc_space
   └─ Optimize hot paths
```

**Risks & Mitigation**:
- Risk: Tests flaky in CI → Mitigation: Retry logic, generous timeouts, parallel tests
- Risk: Release binary size → Mitigation: Strip debug symbols, UPX compression, target ~10MB

**Deliverables**:
- [ ] Integration test suite with >90% coverage
- [ ] README, ARCHITECTURE, CONTRIBUTING docs
- [ ] CI/CD pipeline for multi-platform builds
- [ ] Docker images for easy deployment

---

## 2. Protocol v1 Detailed Design

### 2.1 Binary Frame Format

**Frame Structure** (Byte-level):
```
+--------+--------+--------+----------+----------+----------+
| Ver(1) | Chan(1)| Type(1)| Len(4)   | Payload  | CRC32(4) |
+--------+--------+--------+----------+----------+----------+
| 0x01   | 0x00   | 0x01   | 0x000010 | <N bytes>| 0xA5B3.. |
+--------+--------+--------+----------+----------+----------+

Total header size: 7 bytes
Total trailer size: 4 bytes
Maximum payload size: 4MB (4,194,304 bytes)
Maximum frame size: 4MB + 11 bytes
```

**Field Descriptions**:
- **Version (1 byte)**: Protocol version, currently 0x01
- **Channel (1 byte)**: Logical channel (0=control, 1=shell, 2=file, 3-255=reserved)
- **Type (1 byte)**: Message type (0x01=DATA, 0x02=ACK, ..., 0x06=RESET)
- **Length (4 bytes)**: Payload length in bytes (big-endian)
- **Payload (N bytes)**: Application data
- **CRC32 (4 bytes)**: CRC32 checksum of header + payload (big-endian)

### 2.2 Channel Allocation Strategy

**Channel 0: Control**
- Session management (create, destroy, list)
- Heartbeat and keepalive
- Capability negotiation
- Error reporting

**Channel 1: Shell PTY**
- Bidirectional streaming
- Terminal resize events
- Exit status notifications

**Channel 2: File Transfer**
- Upload/download requests
- Chunk streaming
- Progress updates

**Channel 3-255: Reserved**
- Future: port forwarding (SSH-style)
- Future: SOCKS proxy
- Future: multiplexed sub-protocols

### 2.3 Version Negotiation Mechanism

**Handshake Flow**:
```
1. Client connects via TLS/WebSocket

2. Client -> Server:
   HELLO {
     version: 1,
     features: [SHELL, FILE],
     client_info: {os: "linux", arch: "amd64"}
   }

3. Server -> Client:
   OK {
     version: 1,
     features: [SHELL, FILE],
     session_id: "uuid-v4",
     server_info: {os: "linux", arch: "amd64"}
   }

   OR

   ERROR {
     code: 400,
     message: "unsupported protocol version"
   }
```

**Backward Compatibility**:
- If client version > server version: use server version
- If client version < server version: use client version
- If no common version: reject connection
- Future: feature flags for optional capabilities

### 2.4 Message Types

**Channel 0 Messages (Control)**:
```go
type HelloMessage struct {
    Version   uint8
    Features  []string
    ClientInfo ClientInfo
}

type OKMessage struct {
    Version    uint8
    Features   []string
    SessionID  string
    ServerInfo ServerInfo
}

type HeartbeatMessage struct {
    Timestamp int64
}

type CreateSessionMessage struct {
    ShellPath string
    Args      []string
    Env       map[string]string
    Rows      uint16
    Cols      uint16
}

type SessionCreatedMessage struct {
    SessionID  uint64
    PTYType    string // "xterm", "xterm-256color", "conpty"
}

type DestroySessionMessage struct {
    SessionID uint64
}

type ErrorMessage struct {
    Code    int
    Message string
}
```

**Channel 1 Messages (Shell)**:
```go
type ShellDataMessage struct {
    SessionID uint64
    Data      []byte
}

type ShellResizeMessage struct {
    SessionID uint64
    Rows      uint16
    Cols      uint16
}

type ShellExitMessage struct {
    SessionID uint64
    ExitCode  int
    Signal    string // Unix only
}
```

**Channel 2 Messages (File)**:
```go
type UploadRequestMessage struct {
    Path     string
    Size     int64
    Mode     uint32
    Checksum string // "sha256:abc123..."
}

type UploadAcceptMessage struct {
    Offset    int64
    ChunkSize int
}

type UploadDataMessage struct {
    Offset int64
    Data   []byte
    CRC32  uint32
}

type UploadCompleteMessage struct {
    FinalChecksum string
}
```

### 2.5 Serialization

**Encoding**: Binary (custom) for efficiency
**Alternative**: Protobuf (v2 consideration)

**Custom Binary Encoding**:
```go
func Encode(msg Message) []byte {
    buf := new(bytes.Buffer)

    // Write header
    buf.WriteByte(msg.Version())
    buf.WriteByte(msg.Channel())
    buf.WriteByte(msg.Type())

    // Write payload
    payload := msg.MarshalBinary()
    binary.Write(buf, binary.BigEndian, uint32(len(payload)))
    buf.Write(payload)

    // Write CRC32
    crc := crc32.ChecksumIEEE(buf.Bytes())
    binary.Write(buf, binary.BigEndian, crc)

    return buf.Bytes()
}

func Decode(data []byte) (Message, error) {
    if len(data) < 11 {
        return nil, ErrInvalidFrame
    }

    // Verify CRC32
    crc := binary.BigEndian.Uint32(data[len(data)-4:])
    if crc != crc32.ChecksumIEEE(data[:len(data)-4]) {
        return nil, ErrCRCMismatch
    }

    // Read header
    version := data[0]
    channel := data[1]
    msgType := data[2]
    length := binary.BigEndian.Uint32(data[3:7])
    payload := data[7 : 7+length]

    // Create message
    msg := NewMessage(version, channel, msgType)
    if err := msg.UnmarshalBinary(payload); err != nil {
        return nil, err
    }

    return msg, nil
}
```

---

## 3. Code Structure Planning

### 3.1 Directory Layout

```
vshell/
├── cmd/
│   ├── vshell-server/
│   │   └── main.go               # Server CLI entrypoint
│   └── vshell-client/
│       └── main.go               # Client CLI entrypoint
├── pkg/
│   ├── transport/
│   │   ├── tls.go                # TLS server/client
│   │   ├── tls_client.go         # Client-specific TLS logic
│   │   ├── websocket.go          # WebSocket wrapper
│   │   └── transport.go          # Transport interface
│   ├── protocol/
│   │   ├── frame.go              # Binary frame encoding/decoding
│   │   ├── message.go            # Message types and serialization
│   │   ├── channel.go            # Channel multiplexing
│   │   └── handshake.go          # Version negotiation
│   ├── auth/
│   │   ├── mtls.go               # Mutual TLS authentication
│   │   ├── jwt.go                # JWT token validation (optional)
│   │   └── auth.go               # Authentication interface
│   ├── session/
│   │   ├── manager.go            # Session lifecycle management
│   │   ├── shell_session.go      # Shell session state
│   │   ├── file_session.go       # File transfer session state
│   │   └── heartbeat.go          # Heartbeat management
│   ├── shell/
│   │   ├── shell.go              # Shell interface
│   │   ├── unix.go               # Unix PTY implementation
│   │   ├── windows.go            # Windows ConPTY implementation
│   │   ├── windows_pipe.go       # Windows fallback (named pipes)
│   │   └── windows_sys.go        # Windows API bindings
│   ├── file/
│   │   ├── transfer.go           # File transfer logic
│   │   ├── upload.go             # Upload handler
│   │   ├── download.go           # Download handler
│   │   └── checksum.go           # CRC32/SHA256 utilities
│   └── logging/
│       ├── logger.go             # Structured logging
│       └── metrics.go            # Metrics collection
├── internal/
│   └── server/
│       ├── server.go             # Server core logic
│       ├── handler.go            # Connection handler
│       └── config.go             # Server configuration
├── tests/
│   ├── integration/
│   │   ├── shell_test.go         # Shell E2E tests
│   │   ├── file_test.go          # File transfer E2E tests
│   │   └── reconnect_test.go     # Reconnection tests
│   └── benchmark/
│       ├── throughput_test.go    # Performance benchmarks
│       └── concurrency_test.go   # Concurrency tests
├── docs/
│   ├── ARCHITECTURE.md           # Architecture overview
│   ├── PROTOCOL.md               # Protocol specification
│   ├── API.md                    # API reference
│   └── DEPLOYMENT.md             # Deployment guide
├── .github/
│   └── workflows/
│       └── ci.yml                # GitHub Actions CI
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
└── README.md
```

### 3.2 Module Responsibilities

**cmd/vshell-server**:
- Parse CLI flags (cobra)
- Load TLS certificates
- Initialize server
- Handle shutdown signals

**cmd/vshell-client**:
- Parse CLI flags (cobra)
- Load client certificates
- Connect to server
- Interactive shell loop

**pkg/transport**:
- TLS server/client implementation
- WebSocket fallback
- Connection pooling
- net.Conn interface abstraction

**pkg/protocol**:
- Binary frame encoding/decoding
- Message serialization
- Channel multiplexing
- Version negotiation

**pkg/auth**:
- mTLS certificate verification
- JWT token validation (optional)
- Authentication interface

**pkg/session**:
- Session lifecycle (create, manage, destroy)
- Heartbeat management
- State tracking

**pkg/shell**:
- Shell interface (cross-platform)
- Unix PTY (creack/pty)
- Windows ConPTY
- Windows fallback (named pipes)

**pkg/file**:
- File upload/download
- Chunking and resume
- Checksum verification
- Progress reporting

**pkg/logging**:
- Structured logging (zerolog)
- Metrics collection
- Log rotation

---

## 4. Dependency Choice Details

### 4.1 CLI Library: Cobra vs urfave/cli

**Recommendation**: **Cobra**

**Comparison**:
| Feature | Cobra | urfave/cli v2 |
|---------|-------|---------------|
| Subcommands | Excellent | Good |
| Flag handling | Excellent (pflag) | Good |
| Help generation | Excellent | Good |
| Shell completion | Excellent (bash, zsh, fish, PowerShell) | Good (bash, zsh) |
| Project adoption | Very high (kubectl, docker, github cli) | Medium |
| Community support | Very active | Active |
| Documentation | Excellent | Good |
| Learning curve | Medium | Easy |

**Why Cobra**:
1. **Industry standard**: Used by kubectl, docker, gh, and many more
2. **Rich ecosystem**: Viper for config, Cobra generator tool
3. **Future-proof**: Better support for complex CLIs with nested subcommands
4. **Completion**: Automatic shell completion for all major shells
5. **Battle-tested**: Proven in production at massive scale

**Cobra Project Structure**:
```go
// cmd/root.go
package cmd

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "vshell-client",
    Short: "vShell remote shell client",
    Long:  `A cross-platform remote shell client with file transfer capabilities.`,
}

func Execute() error {
    return rootCmd.Execute()
}

// cmd/connect.go
package cmd

var connectCmd = &cobra.Command{
    Use:   "connect <server>",
    Short: "Connect to a vShell server",
    Args:  cobra.ExactArgs(1),
    Run:   runConnect,
}

func init() {
    rootCmd.AddCommand(connectCmd)
    connectCmd.Flags().String("cert", "", "Client certificate file")
    connectCmd.Flags().String("key", "", "Client private key file")
    connectCmd.Flags().String("ca", "", "CA certificate file")
    connectCmd.Flags().Bool("insecure", false, "Skip certificate verification")
}

// cmd/upload.go
var uploadCmd = &cobra.Command{
    Use:   "upload <local> <remote>",
    Short: "Upload a file to the remote server",
    Args:  cobra.ExactArgs(2),
    Run:   runUpload,
}
```

### 4.2 Unix PTY: creack/pty

**Why creack/pty**:
1. **Simplicity**: Single function call `pty.Start(cmd)`
2. **Cross-Unix**: Works on Linux, macOS, BSD, Solaris
3. **Maintained**: Active maintenance, 1000+ stars
4. **Well-documented**: Clear examples and README
5. **No dependencies**: Minimal footprint

**Usage Pattern**:
```go
import (
    "os/exec"
    "github.com/creack/pty"
)

func startShell() (*os.File, error) {
    cmd := exec.Command("/bin/bash")
    return pty.Start(cmd)
}

func resizeShell(f *os.File, cols, rows uint16) error {
    winsize := &pty.Winsize{Cols: cols, Rows: rows}
    return pty.Setsize(f, winsize)
}
```

### 4.3 WebSocket: gorilla/websocket

**Why gorilla/websocket**:
1. **Complete implementation**: RFC 6455 compliant
2. **Production-ready**: Used in production by many companies
3. **Feature-rich**: TLS, compression, subprotocols
4. **Well-documented**: Extensive documentation and examples
5. **Active community**: 20k+ stars, frequent updates

**Client/Server Pattern**:
```go
// Server
import (
    "net/http"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    CheckOrigin: func(r *http.Request) bool {
        return true // Configure for production
    },
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("upgrade error: %v", err)
        return
    }
    defer conn.Close()

    // Use conn as net.Conn via wrapper
    wsConn := &WSConn{conn}
    handleConnection(wsConn)
}

// Client
func dialWebSocket(url string) (*websocket.Conn, error) {
    dialer := websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
        TLSClientConfig:  &tls.Config{InsecureSkipVerify: false},
    }

    conn, _, err := dialer.Dial(url, nil)
    return conn, err
}
```

### 4.4 Logging: zerolog

**Why zerolog**:
1. **Zero-allocation**: Minimal performance overhead
2. **Structured**: JSON output by default
3. **Fast**: Benchmarks show 10x faster than logrus
4. **Level handling**: TRACE, DEBUG, INFO, WARN, ERROR
5. **Contextual**: Easy to add context fields

**Usage**:
```go
import "github.com/rs/zerolog/log"

func main() {
    // Initialize
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    // Use
    log.Info().
        Str("conn_id", "abc123").
        Str("session_id", "sess456").
        Msg("client connected")

    log.Error().
        Err(err).
        Str("path", "/tmp/file").
        Msg("file upload failed")
}
```

### 4.5 Additional Dependencies

**Testing**:
- `github.com/stretchr/testify` - Assertions and mocking
- `github.com/golang/mock` - Mock generation

**Utilities**:
- `golang.org/x/term` - Terminal raw mode
- `golang.org/x/crypto` - SSH key parsing (for compatibility)

**Build**:
- `github.com/goreleaser/goreleaser` - Cross-platform releases

---

## 5. Windows PTY Strategy

### 5.1 ConPTY API Usage

**Windows 10 1809+ (Build 17763+)**:

**Core APIs**:
```go
// pkg/shell/windows_sys.go

type _COORD struct {
    X, Y int16
}

type _HANDLE syscall.Handle

var (
    kernel32 = syscall.NewLazyDLL("kernel32.dll")

    procCreatePseudoConsole = kernel32.NewProc("CreatePseudoConsole")
    procResizePseudoConsole = kernel32.NewProc("ResizePseudoConsole")
    procClosePseudoConsole  = kernel32.NewProc("ClosePseudoConsole")
)

func createPseudoConsole(size _COORD, hInput, hOutput _HANDLE) (_HANDLE, error) {
    var hPC _HANDLE
    ret, _, err := procCreatePseudoConsole.Call(
        uintptr(unsafe.Pointer(&size)),
        uintptr(hInput),
        uintptr(hOutput),
        0,
        uintptr(unsafe.Pointer(&hPC)),
    )
    if ret != 0 {
        return 0, err
    }
    return hPC, nil
}

func resizePseudoConsole(hPC _HANDLE, size _COORD) error {
    ret, _, err := procResizePseudoConsole.Call(
        uintptr(hPC),
        uintptr(unsafe.Pointer(&size)),
    )
    if ret != 0 {
        return err
    }
    return nil
}

func closePseudoConsole(hPC _HANDLE) {
    procClosePseudoConsole.Call(uintptr(hPC))
}
```

**Implementation Steps**:
1. Create two named pipes (input + output)
2. Call `CreatePseudoConsole` with pipe handles
3. Initialize `STARTUPINFOEX` with `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`
4. Launch process with `CreateProcess` + `EXTENDED_STARTUPINFO_PRESENT`
5. Read from output pipe, write to input pipe
6. Call `ResizePseudoConsole` on terminal resize

**Complete Example**:
```go
// pkg/shell/windows.go

type WindowsShell struct {
    pty        syscall.Handle
    process    syscall.Handle
    thread     syscall.Handle
    inputPipe  syscall.Handle
    outputPipe syscall.Handle
    size       pty.Winsize
}

func NewWindowsShell(cmd string, args []string, opts ShellOptions) (*WindowsShell, error) {
    // 1. Create named pipes
    var hInputRead, hInputWrite syscall.Handle
    var hOutputRead, hOutputWrite syscall.Handle

    err := createPipe(&hInputRead, &hInputWrite)
    if err != nil {
        return nil, err
    }

    err = createPipe(&hOutputRead, &hOutputWrite)
    if err != nil {
        return nil, err
    }

    // 2. Create pseudo console
    size := _COORD{X: int16(opts.Cols), Y: int16(opts.Rows)}
    hPC, err := createPseudoConsole(size, hInputRead, hOutputWrite)
    if err != nil {
        return nil, err
    }

    // 3. Initialize startup info
    var si syscall.StartupInfo
    si.Cb = uint32(unsafe.Sizeof(si))
    si.Flags = syscall.STARTF_USESTDHANDLES

    var siEx syscall.StartupInfoEx
    siEx.StartupInfo = si
    siEx.Cb = uint32(unsafe.Sizeof(siEx))

    // 4. Initialize attribute list
    var attrListSize uintptr
    procInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrListSize)))

    attrList := make([]byte, attrListSize)
    procInitializeProcThreadAttributeList.Call(uintptr(unsafe.Pointer(&attrList[0])), 1, 0, uintptr(unsafe.Pointer(&attrListSize)))

    procUpdateProcThreadAttribute.Call(
        uintptr(unsafe.Pointer(&attrList[0])),
        0,
        _PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
        uintptr(hPC),
        uintptr(unsafe.Sizeof(hPC)),
        0, 0,
    )

    siEx.AttributeList = (*syscall.ProcThreadAttributeList)(unsafe.Pointer(&attrList[0]))

    // 5. Create process
    var pi syscall.ProcessInformation
    cmdLine := syscall.StringToUTF16Ptr(cmd + " " + strings.Join(args, " "))

    err = syscall.CreateProcess(
        nil,
        cmdLine,
        nil,
        nil,
        false,
        syscall.EXTENDED_STARTUPINFO_PRESENT,
        nil,
        nil,
        &siEx.StartupInfo,
        &pi,
    )
    if err != nil {
        closePseudoConsole(hPC)
        return nil, err
    }

    return &WindowsShell{
        pty:        hPC,
        process:    pi.Process,
        thread:     pi.Thread,
        inputPipe:  hInputWrite,
        outputPipe: hOutputRead,
        size:       pty.Winsize{Cols: opts.Cols, Rows: opts.Rows},
    }, nil
}

func (s *WindowsShell) Read(p []byte) (n int, err error) {
    var done uint32
    err = syscall.ReadFile(s.outputPipe, p, &done, nil)
    return int(done), err
}

func (s *WindowsShell) Write(p []byte) (n int, err error) {
    var done uint32
    err = syscall.WriteFile(s.inputPipe, p, &done, nil)
    return int(done), err
}

func (s *WindowsShell) Resize(cols, rows uint16) error {
    size := _COORD{X: int16(cols), Y: int16(rows)}
    return resizePseudoConsole(s.pty, size)
}

func (s *WindowsShell) Close() error {
    closePseudoConsole(s.pty)
    syscall.CloseHandle(s.inputPipe)
    syscall.CloseHandle(s.outputPipe)
    syscall.CloseHandle(s.process)
    syscall.CloseHandle(s.thread)
    return nil
}

func (s *WindowsShell) Wait() (*ExitStatus, error) {
    var exitCode uint32
    syscall.WaitForSingleObject(s.process, syscall.INFINITE)
    syscall.GetExitCodeProcess(s.process, &exitCode)
    return &ExitStatus{Code: int(exitCode)}, nil
}
```

### 5.2 Fallback: Named Pipes (Windows 7/8)

**Limitations**:
- No PTY emulation
- No color support
- No window resize
- Limited interactive capabilities

**Implementation**:
```go
// pkg/shell/windows_pipe.go

type WindowsPipeShell struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.Reader
    stderr io.Reader
}

func NewWindowsPipeShell(cmd string, args []string, opts ShellOptions) (*WindowsPipeShell, error) {
    c := exec.Command(cmd, args...)

    stdin, err := c.StdinPipe()
    if err != nil {
        return nil, err
    }

    stdout, err := c.StdoutPipe()
    if err != nil {
        return nil, err
    }

    stderr, err := c.StderrPipe()
    if err != nil {
        return nil, err
    }

    if err := c.Start(); err != nil {
        return nil, err
    }

    return &WindowsPipeShell{
        cmd:    c,
        stdin:  stdin,
        stdout: stdout,
        stderr: stderr,
    }, nil
}

func (s *WindowsPipeShell) Read(p []byte) (n int, err error) {
    // Merge stdout and stderr
    // Note: This is a limitation - can't distinguish between them
    return s.stdout.Read(p)
}

func (s *WindowsPipeShell) Write(p []byte) (n int, err error) {
    return s.stdin.Write(p)
}

func (s *WindowsPipeShell) Resize(cols, rows uint16) error {
    // Not supported
    return nil
}

func (s *WindowsPipeShell) Close() error {
    s.stdin.Close()
    return s.cmd.Process.Kill()
}

func (s *WindowsPipeShell) Wait() (*ExitStatus, error) {
    err := s.cmd.Wait()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return &ExitStatus{Code: exitErr.ExitCode()}, nil
        }
        return nil, err
    }
    return &ExitStatus{Code: 0}, nil
}
```

### 5.3 Windows Version Detection

**Method**: Registry (reliable) or RtlGetVersion (accurate)

```go
// pkg/shell/windows_version.go

func getWindowsVersion() (major, minor, build uint32) {
    key, err := registry.OpenKey(registry.LOCAL_MACHINE,
        `SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
        registry.QUERY_VALUE)
    if err != nil {
        return 0, 0, 0
    }
    defer key.Close()

    major, _, _ = key.GetIntegerValue("CurrentMajorVersionNumber")
    minor, _, _ = key.GetIntegerValue("CurrentMinorVersionNumber")

    // Build number varies by Windows version
    buildStr, _, _ := key.GetStringValue("CurrentBuildNumber")
    build = parseBuildNumber(buildStr)

    return major, minor, build
}

func supportsConPTY() bool {
    major, _, build := getWindowsVersion()

    // ConPTY available on Windows 10 1809+ (build 17763+)
    if major >= 10 && build >= 17763 {
        return true
    }

    // Windows 11 always supports ConPTY
    if major > 10 {
        return true
    }

    return false
}
```

### 5.4 Testing Strategy

**Test Matrix**:
| Windows Version | ConPTY | Fallback | Priority |
|-----------------|--------|----------|----------|
| Windows 11 | Yes | - | High |
| Windows 10 21H2 | Yes | - | High |
| Windows 10 1809 | Yes | - | Medium |
| Windows 10 1709 | No | Yes | Low |
| Windows 8.1 | No | Yes | Low |
| Windows 7 SP1 | No | Yes | Low |

**Testing Approach**:
1. Manual testing on real hardware (Windows 11, Windows 10)
2. CI: GitHub Actions windows-latest (Windows Server 2022)
3. Community testing: Older Windows versions
4. Document minimum supported version clearly

---

## 6. Testing Strategy

### 6.1 Unit Tests

**Coverage Goals**: >80% overall, >90% for critical paths

**Key Test Files**:
```
pkg/transport/tls_test.go
pkg/protocol/frame_test.go
pkg/protocol/message_test.go
pkg/session/manager_test.go
pkg/shell/unix_test.go
pkg/shell/windows_test.go
pkg/file/transfer_test.go
```

**Example Unit Test**:
```go
// pkg/protocol/frame_test.go
package protocol

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEncodeDecodeFrame(t *testing.T) {
    tests := []struct {
        name    string
        msg     Message
    }{
        {
            name: "data message",
            msg: &DataMessage{
                Channel: 1,
                Data:    []byte("hello world"),
            },
        },
        {
            name: "heartbeat message",
            msg: &HeartbeatMessage{
                Timestamp: 1234567890,
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Encode
            encoded := Encode(tt.msg)
            require.NotNil(t, encoded)

            // Decode
            decoded, err := Decode(encoded)
            require.NoError(t, err)
            require.NotNil(t, decoded)

            // Compare
            assert.Equal(t, tt.msg, decoded)
        })
    }
}

func TestCRC32Validation(t *testing.T) {
    msg := &DataMessage{Channel: 1, Data: []byte("test")}
    encoded := Encode(msg)

    // Corrupt CRC32
    encoded[len(encoded)-1] ^= 0xFF

    // Decode should fail
    _, err := Decode(encoded)
    assert.Error(t, err)
    assert.Equal(t, ErrCRCMismatch, err)
}
```

### 6.2 Integration Tests

**Test Scenarios**:
1. **Shell Execution**:
   - Connect, create shell, execute command, disconnect
   - Multiple concurrent shells
   - Shell resize
   - Shell exit status

2. **File Transfer**:
   - Upload small file
   - Upload large file (100MB+)
   - Download file
   - Resume upload after disconnect

3. **Reconnection**:
   - Network interruption
   - Server restart
   - Session resume

**Example Integration Test**:
```go
// tests/integration/shell_test.go
package integration

import (
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestShellSessionE2E(t *testing.T) {
    // Start server
    server := NewTestServer(t, Config{
        CertFile: "testdata/server.crt",
        KeyFile:  "testdata/server.key",
    })
    defer server.Close()

    // Connect client
    client := NewTestClient(t, server.Addr(), ClientConfig{
        CertFile: "testdata/client.crt",
        KeyFile:  "testdata/client.key",
    })
    defer client.Close()

    // Create shell
    shell, err := client.CreateShell("/bin/bash", nil, 24, 80)
    require.NoError(t, err)

    // Execute command
    shell.Write([]byte("echo hello\n"))
    time.Sleep(100 * time.Millisecond)

    output := shell.ReadLine()
    require.Contains(t, output, "hello")

    // Resize
    err = shell.Resize(40, 120)
    require.NoError(t, err)

    // Exit
    shell.Write([]byte("exit\n"))
    exitStatus := shell.Wait()
    require.Equal(t, 0, exitStatus.Code)
}

func TestMultipleShells(t *testing.T) {
    server := NewTestServer(t, DefaultConfig())
    defer server.Close()

    client := NewTestClient(t, server.Addr(), DefaultClientConfig())
    defer client.Close()

    // Create 10 shells
    shells := make([]*ShellSession, 10)
    for i := 0; i < 10; i++ {
        shell, err := client.CreateShell("/bin/bash", nil, 24, 80)
        require.NoError(t, err)
        shells[i] = shell
    }

    // Execute commands concurrently
    for i, shell := range shells {
        go func(idx int, s *ShellSession) {
            s.Write([]byte("echo test\n"))
            s.Write([]byte("exit\n"))
        }(i, shell)
    }

    // Wait for all
    for _, shell := range shells {
        exitStatus := shell.Wait()
        require.Equal(t, 0, exitStatus.Code)
    }
}
```

### 6.3 Performance Tests

**Benchmarks**:
```go
// tests/benchmark/throughput_test.go
package benchmark

import "testing"

func BenchmarkFileUpload(b *testing.B) {
    server := NewBenchmarkServer(b)
    defer server.Close()

    client := NewBenchmarkClient(b, server.Addr())
    defer client.Close()

    data := make([]byte, 1024*1024) // 1MB

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        client.UploadFile("test.dat", data)
    }

    b.ReportMetric(float64(len(data)*b.N)/1e9, "GB")
    b.ReportMetric(float64(len(data)*b.N)/b.Elapsed().Seconds()/1e6, "MB/s")
}

func BenchmarkShellLatency(b *testing.B) {
    server := NewBenchmarkServer(b)
    defer server.Close()

    client := NewBenchmarkClient(b, server.Addr())
    defer client.Close()

    shell := client.CreateShell("/bin/bash", nil, 24, 80)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        shell.Write([]byte("echo test\n"))
        shell.ReadLine()
    }
}
```

### 6.4 CI/CD Pipeline

**GitHub Actions**:
```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
        go: ['1.26']

    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: ${{ matrix.go }}

      - name: Build
        run: go build ./...

      - name: Test
        run: go test -race -coverprofile=coverage.txt -covermode=atomic ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.txt

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: golangci/golangci-lint-action@v3
        with:
          version: latest

  release:
    needs: [test, lint]
    runs-on: ubuntu-latest
    if: startsWith(github.ref, 'refs/tags/')
    steps:
      - uses: actions/checkout@v3

      - uses: goreleaser/goreleaser-action@v4
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## 7. Post-MVP Evolution Path

### 7.1 Design for Extensibility

**Protocol Versioning**:
- Reserve version numbers: 1-15
- Each version has a clear upgrade path
- Backward compatibility: server supports current + previous version

**Channel Reservation**:
- 0-2: Core functionality (v1)
- 3-15: Reserved for official features (v2+)
- 16-255: User-defined channels

**Message Type Extension**:
- 0x01-0x0F: Core message types (v1)
- 0x10-0x7F: Reserved for official features (v2+)
- 0x80-0xFF: User-defined message types

**Metadata Fields**:
- Each message includes optional metadata map
- Future: compression, encryption flags

### 7.2 v2 Protocol (Post-MVP)

**Timeline**: 3-6 months after MVP

**Features**:
1. **Compression**:
   - zlib/zstd compression per message
   - Negotiated during handshake
   - Compression ratio in metadata

2. **Multiplexed Streams**:
   - Port forwarding (like SSH -L)
   - SOCKS5 proxy
   - Multiple shells per session

3. **Enhanced Auth**:
   - OAuth2/OIDC integration
   - API key authentication
   - Session tokens with expiration

4. **Streaming API**:
   - Server-sent events
   - Bidirectional streaming for large outputs

**Protocol Extension**:
```
v2 Frame Format:
+--------+--------+--------+----------+----------+----------+----------+
| Ver(1) | Chan(1)| Type(1)| Flags(1) | Len(4)   | Payload  | CRC32(4) |
+--------+--------+--------+----------+----------+----------+----------+

Flags:
  0x01 = Compressed (zstd)
  0x02 = Encrypted (AES-GCM)
  0x04 = Stream start
  0x08 = Stream end
```

### 7.3 QUIC Migration (Long-term)

**Timeline**: 6-12 months after MVP

**Benefits**:
- Built-in multiplexing (no need for custom channel layer)
- 0-RTT connection establishment
- Better mobile/wireless performance
- No head-of-line blocking

**Migration Strategy**:
1. Implement QUIC transport alongside TCP-TLS
2. Client advertises QUIC support via DNS TXT or well-known endpoint
3. Gradual migration: new clients use QUIC, old clients fallback to TCP
4. Deprecate TCP-TLS after 6 months

**Library**: `github.com/lucas-clemente/quic-go`

**Architecture**:
```go
// pkg/transport/quic.go
type QUICTransport struct {
    listener quic.Listener
}

func (t *QUICTransport) Accept() (net.Conn, error) {
    session, err := t.listener.Accept(context.Background())
    if err != nil {
        return nil, err
    }

    stream, err := session.OpenStreamSync(context.Background())
    if err != nil {
        return nil, err
    }

    return &QUICConn{session, stream}, nil
}
```

---

## 8. Risk Assessment & Mitigation

### 8.1 Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Protocol design flaw discovered late | Medium | High | Week 1 review, freeze protocol, document assumptions |
| Windows ConPTY instability | Medium | Medium | Extensive testing, fallback implementation, version detection |
| PTY permissions in containers | Low | Medium | Document requirements, test in Docker, provide workarounds |
| Memory leaks in long-running sessions | Medium | High | pprof monitoring, stress testing, resource cleanup audit |
| WebSocket fallback performance | Low | Low | Benchmark, optimize if needed, document limitations |
| TLS configuration incompatibilities | Low | Medium | Test with multiple implementations (OpenSSL, BoringSSL) |
| Large file transfer memory usage | Medium | High | Stream chunks, never load entire file in memory, strict limits |

### 8.2 Schedule Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Week 3 Windows PTY overruns | High | High | Start Windows research in Week 2, allocate buffer time |
| Integration testing delays | Medium | Medium | Write tests incrementally, not all in Week 6 |
| Scope creep | High | High | Strict feature freeze after Week 1, reject nice-to-haves |
| Team availability | Medium | High | Cross-training, documentation, no single points of failure |
| Third-party dependency issues | Low | Low | Vendor dependencies, test with locked versions |

### 8.3 Scope Management

**MVP Scope (MUST)**:
- [x] TLS transport
- [x] WebSocket fallback
- [x] Shell execution (Unix + Windows)
- [x] File transfer with resume
- [x] Session management
- [x] Automatic reconnection
- [x] Basic CLI

**Post-MVP (FUTURE)**:
- [ ] Port forwarding
- [ ] SOCKS5 proxy
- [ ] Compression
- [ ] OAuth2/OIDC
- [ ] Web UI
- [ ] Mobile client
- [ ] Plugin system

**Rejection Criteria**:
- "We need SSH protocol compatibility" → Use OpenSSH instead
- "Add built-in text editor" → Outside scope, use existing editors
- "Support Windows XP" → Not worth maintenance burden

### 8.4 Operational Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Security vulnerability | Low | Critical | Security audit, dependency scanning, responsible disclosure |
| Performance in production | Medium | High | Load testing, benchmarking, monitoring |
| Backward compatibility breaking | Low | High | Semantic versioning, protocol versioning, deprecation policy |
| Documentation outdated | Medium | Medium | Generate docs from code, periodic review, community feedback |

---

## 9. Success Metrics

### 9.1 MVP Success Criteria

**Functional**:
- [ ] Shell execution works on Linux + Windows 10+
- [ ] File upload/download works with resume
- [ ] Reconnection recovers session state
- [ ] CLI connects and executes commands

**Non-Functional**:
- [ ] Throughput: >50 MB/s file transfer on localhost
- [ ] Latency: <10ms round-trip for shell commands
- [ ] Concurrency: 50+ sessions without degradation
- [ ] Uptime: 24+ hours without memory leaks
- [ ] Test coverage: >80%

### 9.2 Post-MVP Metrics

**Adoption**:
- GitHub stars: >500 in 3 months
- Docker pulls: >1000 in 1 month
- Community contributions: >5 PRs

**Performance**:
- Throughput: >100 MB/s (QUIC)
- Latency: <5ms (QUIC)
- Concurrency: 100+ sessions

---

## Appendix A: Project Initialization

**Repository Setup**:
```bash
# Initialize
git init
git remote add origin https://github.com/yourname/vshell.git

# Module init
go mod init github.com/yourname/vshell

# Directory structure
mkdir -p cmd/vshell-server cmd/vshell-client
mkdir -p pkg/{transport,protocol,auth,session,shell,file,logging}
mkdir -p internal/server
mkdir -p tests/{integration,benchmark}
mkdir -p docs

# Initial dependencies
go get github.com/spf13/cobra@latest
go get github.com/gorilla/websocket@latest
go get github.com/creack/pty@latest
go get github.com/rs/zerolog@latest
go get github.com/stretchr/testify@latest
```

**Makefile**:
```makefile
.PHONY: build test lint clean

build:
	go build -o bin/vshell-server ./cmd/vshell-server
	go build -o bin/vshell-client ./cmd/vshell-client

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

docker:
	docker build -t vshell:latest .
```

---

## Appendix B: Resources

**Documentation**:
- [Gorilla WebSocket Docs](https://context7.com/gorilla/websocket/)
- [Cobra User Guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md)
- [creack/pty Wiki](https://deepwiki.com/creack/pty)

**Windows ConPTY**:
- [Microsoft ConPTY Documentation](https://learn.microsoft.com/en-us/windows/console/createpseudoconsole)
- [ConPTY Samples](https://github.com/microsoft/terminal/tree/main/samples)

**Best Practices**:
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Standard Go Project Layout](https://medium.com/golang-learn/standard-go-project-layout-c5c19e7e9c4b)

---

**End of MVP Implementation Guide**

This document provides a comprehensive roadmap for delivering a production-ready MVP in 6 weeks. Follow the weekly breakdown, manage risks proactively, and resist scope creep. Focus on delivering a solid foundation that can evolve based on real-world feedback.
