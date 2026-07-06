package cleanup

import (
	"os"
	"os/signal"

	"github.com/Cloud-Foundations/Dominator/lib/list"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func cleanupOnSignal(cf *CleanupFunctions, exitCode int, sig ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig...)
	go func() {
		<-ch
		cf.HardCleanup()
		os.Exit(exitCode)
	}()
}

func newCleanupFunctions(logger log.DebugLogger) *CleanupFunctions {
	return &CleanupFunctions{
		functionList: list.New[Function](),
		logger:       logger,
	}
}

func (cf *CleanupFunctions) add(fn Function) {
	cf.functionList.PushFront(fn)
}

func (cf *CleanupFunctions) cleanup() error {
	var err error
	cf.functionList.IterateValues(func(fn Function) bool {
		if e := fn(); e != nil {
			err = e
			return false
		}
		return true
	})
	return err
}

func (cf *CleanupFunctions) hardCleanup() error {
	var err error
	cf.functionList.IterateValues(func(fn Function) bool {
		if e := fn(); e != nil {
			if err != nil {
				err = e
			}
			cf.logger.Println(e)
		}
		return true
	})
	return err
}
