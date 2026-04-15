//go:build !windows
// +build !windows

package shell

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// PTY implements Shell using Unix PTY
type PTY struct {
	pty     *os.File
	cmd     *exec.Cmd
	winsize *pty.Winsize
}

// newUnixShell creates a new Unix shell with PTY support
func newUnixShell(opts Options) (Shell, error) {
	var cmd *exec.Cmd

	if opts.Command == "" {
		// Use login shell
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell, "-l")
	} else {
		cmd = exec.Command(opts.Command, opts.Args...)
	}

	// Set environment
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Set working directory
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Start PTY
	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(opts.Cols),
		Rows: uint16(opts.Rows),
	})
	if err != nil {
		return nil, err
	}

	return &PTY{
		pty: ptyFile,
		cmd: cmd,
		winsize: &pty.Winsize{
			Cols: uint16(opts.Cols),
			Rows: uint16(opts.Rows),
		},
	}, nil
}

// Read implements io.Reader
func (p *PTY) Read(b []byte) (int, error) {
	return p.pty.Read(b)
}

// Write implements io.Writer
func (p *PTY) Write(b []byte) (int, error) {
	return p.pty.Write(b)
}

// Close closes the PTY
func (p *PTY) Close() error {
	p.pty.Close()
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	return nil
}

// Resize updates the terminal size
func (p *PTY) Resize(cols, rows uint16) error {
	p.winsize.Cols = cols
	p.winsize.Rows = rows
	return pty.Setsize(p.pty, p.winsize)
}

// Wait waits for the process to exit
func (p *PTY) Wait() (*ExitStatus, error) {
	err := p.cmd.Wait()
	if err == nil {
		return &ExitStatus{Code: 0}, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			sig := ""
			if status.Signaled() {
				sig = status.Signal().String()
			}
			return &ExitStatus{
				Code:   status.ExitStatus(),
				Signal: sig,
			}, nil
		}
		return &ExitStatus{Code: 1}, nil
	}
	return nil, err
}

// Pid returns the process ID
func (p *PTY) Pid() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// New creates a new shell based on platform
func New(opts Options) (Shell, error) {
	return newUnixShell(opts)
}
