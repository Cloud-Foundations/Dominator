package cleanup

import (
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/list"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type CleanupFunctions struct {
	functionList *list.List[Function]
	logger       log.DebugLogger
}

type Function func() error

// CleanupOnSignal will call the HardCleanup() method for the specified cleanup
// functions when one of the specified signals is caught and then exits the
// programme with the specified exitCode.
func CleanupOnSignal(cf *CleanupFunctions, exitCode int, sig ...os.Signal) {
	cleanupOnSignal(cf, exitCode, sig...)
}

// NewCleanupFunctions creates a container for cleanup functions.
func NewCleanupFunctions(logger log.DebugLogger) *CleanupFunctions {
	return newCleanupFunctions(logger)
}

// Add will a function to the list of cleanup functions.
func (cf *CleanupFunctions) Add(fn Function) {
	cf.add(fn)
}

// Cleanup will call all the cleanup functions, starting with the last added
// (LIFO). If any function returns an error, the cleanup will be terminated
// early and it's error will be returned.
func (cf *CleanupFunctions) Cleanup() error {
	return cf.cleanup()
}

// HardCleanup will call all the cleanup functions, starting with the last added
// (LIFO). If any function returns an error it is logged, but cleanup will
// continue regardless. The error of the first function which returns an error
// will be returned.
func (cf *CleanupFunctions) HardCleanup() error {
	return cf.hardCleanup()
}
