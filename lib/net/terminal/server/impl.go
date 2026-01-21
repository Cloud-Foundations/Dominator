package server

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

const (
	ptyMasterDevice = "/dev/ptmx"
	ttyPrefix       = "/dev/pts/"
)

type fdGetter interface {
	Fd() uintptr
}

type flushWriter interface {
	Flush() error
	io.Writer
}

func copyFromPty(conn flushWriter, pty io.Reader, killed *bool,
	logger log.Logger) {
	buffer := make([]byte, 256)
	for {
		if nRead, err := pty.Read(buffer); err != nil {
			if *killed {
				break
			}
			logger.Printf("error reading from pty: %s", err)
			break
		} else if _, err := conn.Write(buffer[:nRead]); err != nil {
			logger.Printf("error writing to connection: %s\n", err)
			break
		}
		if err := conn.Flush(); err != nil {
			logger.Printf("error flushing connection: %s\n", err)
			break
		}
	}
}

func copyToPty(pty io.Writer, reader io.Reader) error {
	fd := -2
	if getter, ok := pty.(fdGetter); ok {
		fd = int(getter.Fd())
	}
	buffer := make([]byte, 256)
	for {
		if nRead, err := reader.Read(buffer); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		} else {
			if _, err := pty.Write(buffer[:nRead]); err != nil {
				return fmt.Errorf("error writing to pty(%d): %w", fd, err)
			}
		}
	}
}

func openPty() (*os.File, *os.File, error) {
	pty, err := os.OpenFile(ptyMasterDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	doClosePty := true
	defer func() {
		if doClosePty {
			pty.Close() // Best effort.
		}
	}()
	sname, err := ptsname(pty)
	if err != nil {
		return nil, nil, err
	}
	if err := unlockpt(pty); err != nil {
		return nil, nil, err
	}
	tty, err := os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	doClosePty = false
	return pty, tty, nil
}

func ptsname(pty *os.File) (string, error) {
	var n uintptr
	err := wsyscall.Ioctl(int(pty.Fd()), syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&n)))
	if err != nil {
		return "", err
	}
	return ttyPrefix + strconv.Itoa(int(n)), nil
}

func runCommand(g *goroutine.Goroutine, conn FlushReadWriter,
	pty, tty io.ReadWriter, cmd *exec.Cmd, logger log.DebugLogger) error {
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	var err error
	if g == nil {
		err = cmd.Start()
	} else {
		g.Run(func() { err = cmd.Start() })
	}
	if err != nil {
		return err
	}
	killed := false
	go copyFromPty(conn, pty, &killed, logger) // Read from pty until killed.
	// Read from connection, write to pty.
	err = copyToPty(pty, conn)
	killed = true
	cmd.Process.Kill()
	cmd.Wait()
	return err
}

func unlockpt(pty *os.File) error {
	var u uintptr
	// Use TIOCSPTLCK with a zero valued arg to clear the slave pty lock.
	return wsyscall.Ioctl(int(pty.Fd()), syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&u)))
}
