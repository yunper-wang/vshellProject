//go:build windows
// +build windows

package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"golang.org/x/sys/windows"
)

// ConPTY implements Shell using Windows ConPTY (Windows 10+)
type ConPTY struct {
	ptyIn   *os.File
	ptyOut  *os.File
	cmd     *exec.Cmd
	process windows.Handle
	mu      sync.Mutex
	closed  bool
	console uintptr
}

// newWindowsShell creates a new Windows shell with ConPTY support
func newWindowsShell(opts Options) (Shell, error) {
	// Check if ConPTY is available (Windows 10 version 1809+)
	if !isConPTYAvailable() {
		return nil, fmt.Errorf("ConPTY not available, use WindowsPipeShell for legacy Windows")
	}

	var cmd *exec.Cmd
	if opts.Command == "" {
		// Use default shell
		cmd = exec.Command("cmd.exe")
	} else {
		cmd = exec.Command(opts.Command, opts.Args...)
	}

	// Create pseudo console
	cols := opts.Cols
	rows := opts.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	// Create ConPTY
	console, ptyIn, ptyOut, err := createPseudoConsole(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create ConPTY: %w", err)
	}

	// Set working directory
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Set environment
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Start process with ConPTY
	attr := &syscall.ProcAttr{
		Dir:   cmd.Dir,
		Env:   cmd.Env,
		Files: []*os.File{nil, nil, nil},
	}

	pid, _, err := syscall.StartProcess(cmd.Path, cmd.Args, attr)
	if err != nil {
		closePseudoConsole(console)
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	return &ConPTY{
		ptyIn:   ptyIn,
		ptyOut:  ptyOut,
		cmd:     cmd,
		process: windows.Handle(pid),
		console: console,
	}, nil
}

// Read implements io.Reader
func (c *ConPTY) Read(b []byte) (int, error) {
	return c.ptyOut.Read(b)
}

// Write implements io.Writer
func (c *ConPTY) Write(b []byte) (int, error) {
	return c.ptyIn.Write(b)
}

// Close closes the ConPTY
func (c *ConPTY) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.process != 0 {
		windows.TerminateProcess(c.process, 0)
	}
	closePseudoConsole(c.console)
	return nil
}

// Resize updates the terminal size
func (c *ConPTY) Resize(cols, rows uint16) error {
	return resizePseudoConsole(c.console, cols, rows)
}

// Wait waits for the process to exit
func (c *ConPTY) Wait() (*ExitStatus, error) {
	var exitCode uint32
	event, err := windows.WaitForSingleObject(c.process, windows.INFINITE)
	if err != nil {
		return nil, err
	}
	if event == windows.WAIT_OBJECT_0 {
		windows.GetExitCodeProcess(c.process, &exitCode)
	}
	return &ExitStatus{Code: int(exitCode)}, nil
}

// Pid returns the process ID
func (c *ConPTY) Pid() int {
	return int(c.process)
}

// Helper functions for ConPTY

var (
	modKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procCreatePseudoConsole = modKernel32.NewProc("CreatePseudoConsole")
	procClosePseudoConsole  = modKernel32.NewProc("ClosePseudoConsole")
	procResizePseudoConsole = modKernel32.NewProc("ResizePseudoConsole")
)

func isConPTYAvailable() bool {
	return procCreatePseudoConsole.Find() == nil
}

func createPseudoConsole(cols, rows uint16) (uintptr, *os.File, *os.File, error) {
	// Simplified implementation - full implementation would use Windows API
	// This is a placeholder for the actual ConPTY implementation
	return 0, nil, nil, fmt.Errorf("ConPTY implementation requires Windows API integration")
}

func closePseudoConsole(console uintptr) {
	if console != 0 {
		procClosePseudoConsole.Call(console)
	}
}

func resizePseudoConsole(console uintptr, cols, rows uint16) error {
	// Placeholder for resize implementation
	return nil
}

// New creates a new shell based on platform
func New(opts Options) (Shell, error) {
	return newWindowsShell(opts)
}
