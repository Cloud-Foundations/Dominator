package osutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

// HardReboot will try to sync file-system data and then issues a reboot system
// call. It will not block indefinitely on syncing. It doesn't depend on a
// working "reboot" programme. This is a reboot of last resort.
func HardReboot(logger log.Logger) error {
	return hardReboot(logger)
}

// RunCommand will run a command, returning true on success. If there is an
// error, a message is logged and false is returned.
// The name of the command to run must be specified by name.
func RunCommand(logger log.Logger, name string, args ...string) bool {
	return runCommand(logger, name, args...)
}

// RunCommandBackground will run a command in a goroutine and returns a channel
// that will receive a message if the command fails. No message is received if
// the command succeeds. Errors are logged.
// This is useful for commands that start asynchronous work where you will wait
// for completion but want failures to trigger a cancellation on waiting.
func RunCommandBackground(logger log.Logger, name string,
	args ...string) <-chan struct{} {
	return runCommandBackground(logger, name, args...)
}
