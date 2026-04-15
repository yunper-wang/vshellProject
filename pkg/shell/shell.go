package shell

import (
	"io"
)

// Shell interface abstracts shell execution across platforms
// Implementations: UnixShell (Linux/Mac), WindowsShell (Win10+), WindowsPipeShell (legacy)
type Shell interface {
	io.ReadWriteCloser

	// Resize updates the terminal size
	Resize(cols, rows uint16) error

	// Wait waits for the process to exit and returns status
	Wait() (*ExitStatus, error)

	// Pid returns the process ID
	Pid() int
}

// ExitStatus holds the process exit information
type ExitStatus struct {
	Code   int
	Signal string // Unix only
}

// Options holds shell creation options
type Options struct {
	Command string
	Args    []string
	Env     map[string]string
	Dir     string
	Cols    uint16
	Rows    uint16
}

// New creates a new shell based on platform
// Unix: uses creack/pty
// Windows 10+: uses ConPTY
// Older Windows: uses named pipes
type Factory func(opts Options) (Shell, error)
