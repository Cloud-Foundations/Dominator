package filegen

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
)

type programmeGenerator struct {
	logger          log.Logger
	notifierChannel chan<- string
	objectServer    *memory.ObjectServer
	pathname        string
	programmePath   string
}

type programmeResult struct {
	Data         []byte
	SecondsValid uint64
}

func (m *Manager) registerProgrammeForPath(pathname, programmePath string) {
	progGen := &programmeGenerator{
		logger:        m.logger,
		objectServer:  m.objectServer,
		pathname:      pathname,
		programmePath: programmePath,
	}
	progGen.notifierChannel = m.RegisterGeneratorForPath(pathname, progGen)
}

func (progGen *programmeGenerator) Generate(machine mdb.Machine,
	logger log.Logger) ([]byte, time.Time, error) {
	cmd := exec.Command(progGen.programmePath, progGen.pathname)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, time.Time{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, time.Time{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := cmd.Start(); err != nil {
		return nil, time.Time{}, err
	}
	if err := json.WriteWithIndent(stdin, "    ", machine); err != nil {
		return nil, time.Time{},
			fmt.Errorf("error writing machine data to programme: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return nil, time.Time{},
			fmt.Errorf("error closing stdin for programme: %w", err)
	}
	var result programmeResult
	// Read all of the result from stdout and all errors from stderr.
	stdoutReadError := json.Read(stdout, &result)
	stderrBuilder := &strings.Builder{}
	io.Copy(stderrBuilder, stderr)
	stderrData := strings.TrimSpace(stderrBuilder.String())
	if err := cmd.Wait(); err != nil {
		return nil, time.Time{},
			fmt.Errorf("error running: %s, exit code: %d, stderr: %s",
				progGen.programmePath, cmd.ProcessState.ExitCode(), stderrData)
	}
	if stdoutReadError != nil {
		if stderrData == "" {
			return nil, time.Time{},
				fmt.Errorf("error reading result from programme: %w",
					stdoutReadError)
		}
		return nil, time.Time{},
			fmt.Errorf("error reading result from programme: %w, stderr: %s",
				stdoutReadError, stderrData)
	}
	var validUntil time.Time
	if result.SecondsValid > 0 {
		validUntil = time.Now().Add(time.Duration(result.SecondsValid) *
			time.Second)
	}
	return result.Data, validUntil, nil
}
