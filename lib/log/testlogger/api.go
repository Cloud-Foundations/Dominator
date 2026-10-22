package testlogger

import (
	"sync"
	"time"
)

type sprintFunc func(v ...interface{}) string
type sprintfFunc func(format string, v ...interface{}) string

type Logger struct {
	logger    TestLogger
	sprint    sprintFunc
	sprintf   sprintfFunc
	startTime time.Time
	mutex     sync.Mutex // Protect done.
	done      bool       // Set when the test completes.
}

// TestLogger defines an interface for a type that can be used for logging by
// tests. The testing.T type from the standard library satisfies this interface.
type TestLogger interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Log(v ...interface{})
	Logf(format string, v ...interface{})
}

// Cleaner is an interface for registering teardown operations.
// It is satisfied by standard testing types (like *testing.T) to safely
// halt background tasks and prevent panics after a test completes.
type Cleaner interface {
	Cleanup(f func())
}

// New will create a Logger from a TestLogger. The Logger that is created
// satisfies the log.DebugLogger interface and thus may be used widely. It
// serves as an adaptor between the testing.T type from the standard library and
// library code that expects a generic logging type.
// Trailing newlines are removed before calling the TestLogger methods.
func New(logger TestLogger) *Logger {
	return newTestlogger(logger)
}

// NewWithTimestamps is the same as New, except that timestamps (since creating
// the logger) are included in the log messages.
// Trailing newlines are removed before calling the TestLogger methods.
func NewWithTimestamps(logger TestLogger) *Logger {
	return newWithTimestamps(logger)
}

// Debug will call the Log method of the underlying TestLogger, regardless of
// the debug level.
func (l *Logger) Debug(level uint8, v ...interface{}) {
	l.emit(l.sprint(v...))
}

// Debugf is similar to Debug, with formatting support.
func (l *Logger) Debugf(level uint8, format string, v ...interface{}) {
	l.emit(l.sprintf(format, v...))
}

// Debugln is similar to Debug.
func (l *Logger) Debugln(level uint8, v ...interface{}) {
	l.emit(l.sprint(v...))
}

// Fatal will call the Fatal method of the underlying TestLogger.
func (l *Logger) Fatal(v ...interface{}) {
	l.emitFatal(l.sprint(v...))
}

// Fatalf is similar to Fatal, with formatting support.
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.emitFatal(l.sprintf(format, v...))
}

// Fatalln is similar to Fatal.
func (l *Logger) Fatalln(v ...interface{}) {
	l.emitFatal(l.sprint(v...))
}

// Panic will call the Fatal method of the underlying TestLogger and will then
// call panic.
func (l *Logger) Panic(v ...interface{}) {
	s := l.sprint(v...)
	l.emitFatal(s)
	panic(s)
}

// Panicf is similar to Panic, with formatting support.
func (l *Logger) Panicf(format string, v ...interface{}) {
	s := l.sprintf(format, v...)
	l.emitFatal(s)
	panic(s)
}

// Panicln is similar to Panic.
func (l *Logger) Panicln(v ...interface{}) {
	s := l.sprint(v...)
	l.emitFatal(s)
	panic(s)
}

// Print will call the Log method of the underlying TestLogger.
func (l *Logger) Print(v ...interface{}) {
	l.emit(l.sprint(v...))
}

// Printf is similar to Print, with formatting support.
func (l *Logger) Printf(format string, v ...interface{}) {
	l.emit(l.sprintf(format, v...))
}

// Println is similar to Print.
func (l *Logger) Println(v ...interface{}) {
	l.emit(l.sprint(v...))
}
