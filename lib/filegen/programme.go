package filegen

import (
	"os/exec"
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
		return nil, time.Time{}, err
	}
	if err := stdin.Close(); err != nil {
		return nil, time.Time{}, err
	}
	var result programmeResult
	if err := json.Read(stdout, &result); err != nil {
		return nil, time.Time{}, err
	}
	if err := stdout.Close(); err != nil {
		return nil, time.Time{}, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, time.Time{}, err
	}
	var validUntil time.Time
	if result.SecondsValid > 0 {
		validUntil = time.Now().Add(time.Duration(result.SecondsValid) *
			time.Second)
	}
	return result.Data, validUntil, nil
}
