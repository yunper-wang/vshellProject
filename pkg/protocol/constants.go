package protocol

// Protocol version
const (
	Version = 0x01
)

// Channel types
const (
	ChannelControl = 0x00 // Control messages (Hello, Heartbeat, Error)
	ChannelShell   = 0x01 // Shell/PTY data
	ChannelFile    = 0x02 // File transfer
)

// Control message types
const (
	TypeHello      = 0x01
	TypeOk         = 0x02
	TypeHeartbeat  = 0x03
	TypeDisconnect = 0x04
	TypeWindowSize = 0x05 // Window resize for PTY
	TypeError      = 0xFF
)

// Shell message types
const (
	TypeShellData   = 0x00 // Raw terminal data
	TypeShellResize = 0x01
	TypeShellEOF    = 0x02
)

// File message types
const (
	TypeFileRequest  = 0x01 // Upload/Download request
	TypeFileResponse = 0x02 // Server response
	TypeFileData     = 0x03 // File chunk
	TypeFileAck      = 0x04 // Acknowledgment
	TypeFileError    = 0x05
)

// File operation codes
const (
	FileOpUpload   = 0x01
	FileOpDownload = 0x02
)

// Error codes
const (
	ErrUnknown    = 0x01
	ErrAuthFailed = 0x02
	ErrInvalidMsg = 0x03
	ErrNotFound   = 0x04
	ErrPermDenied = 0x05
	ErrIOError    = 0x06
	ErrNoSession  = 0x07
	ErrConnClosed = 0x08
)
