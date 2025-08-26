package builder

import (
	"context"
	"errors"
	"time"
)

func (b *Builder) replaceIdleSlaves(immediateGetNew bool) error {
	if b.slaveDriver == nil {
		return errors.New("no SlaveDriver configured")
	}
	b.slaveDriver.ReplaceIdle(immediateGetNew)
	return nil
}

func makeContext(deadline time.Duration) (context.Context, context.CancelFunc) {
	if deadline < time.Second {
		deadline = 24 * time.Hour
	}
	return context.WithDeadline(context.Background(), time.Now().Add(deadline))
}
