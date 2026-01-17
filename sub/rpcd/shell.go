package rpcd

import (
	"os/exec"
	"syscall"

	tserver "github.com/Cloud-Foundations/Dominator/lib/net/terminal/server"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var noShellCommandMessage = []byte("no shell command configured\n")

func (t *rpcType) Shell(conn *srpc.Conn) error {
	if len(t.config.ShellCommand) < 1 {
		conn.Write(noShellCommandMessage)
		return srpc.ErrorCloseClient
	}
	pty, tty, err := tserver.OpenPty()
	if err != nil {
		return err
	}
	defer pty.Close()
	defer tty.Close()
	t.params.Logger.Printf(
		"Shell(%s) on SRPC connection with tty: %s\n",
		conn.Username(), tty.Name())
	var cmd *exec.Cmd
	if len(t.config.ShellCommand) > 1 {
		cmd = exec.Command(t.config.ShellCommand[0],
			t.config.ShellCommand[1:]...)
	} else {
		cmd = exec.Command(t.config.ShellCommand[0])
	}
	cmd.Env = make([]string, 0)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	err = tserver.RunCommand(t.params.WorkdirGoroutine, conn, pty, tty, cmd,
		t.params.Logger)
	if err == nil {
		t.params.Logger.Printf(
			"Shell(%s) on SRPC connection exited with tty: %s\n",
			conn.Username(), tty.Name())
		return srpc.ErrorCloseClient
	}
	return err
}
