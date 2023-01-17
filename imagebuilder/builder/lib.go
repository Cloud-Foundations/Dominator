package builder

import (
	"errors"
)

func (b *Builder) replaceIdleSlaves(immediateGetNew bool) error {
	if b.slaveDriver == nil {
		return errors.New("no SlaveDriver configured")
	}
	b.slaveDriver.ReplaceIdle(immediateGetNew)
	return nil
}
