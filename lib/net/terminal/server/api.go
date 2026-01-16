package server

import (
	"io"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type FlushReadWriter interface {
	Flush() error
	io.ReadWriter
}

type NamedReadWriteCloser interface {
	Name() string
	io.ReadWriteCloser
}

// OpenPty will open a pseudo-terminal pair, returning each side of the pair,
// the name of the TTY side or an error.
func OpenPty() (pty, tty NamedReadWriteCloser, err error) {
	return openPty()
}

// RunCommand will run a command inside a pseudo-terminal session. Data are
// copied between the connection and the PTY master.
// If g is not nil, the process is started from within the specified Goroutine.
func RunCommand(g *goroutine.Goroutine, conn FlushReadWriter,
	pty, tty io.ReadWriter, cmd *exec.Cmd, logger log.DebugLogger) error {
	return runCommand(g, conn, pty, tty, cmd, logger)
}
