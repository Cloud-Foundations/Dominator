package testlogger

import (
	"fmt"
	"time"
)

func strip(s string) string {
	length := len(s)
	if length < 1 {
		return ""
	}
	if s[length-1] == '\n' {
		return s[:length-1]
	}
	return s
}

func plainSprint(v ...interface{}) string {
	return strip(fmt.Sprint(v...))
}

func plainSprintf(format string, v ...interface{}) string {
	return strip(fmt.Sprintf(format, v...))
}

func newTestlogger(logger TestLogger) *Logger {
	l := &Logger{
		logger:  logger,
		sprint:  plainSprint,
		sprintf: plainSprintf,
	}
	l.stopAfterCompletion()
	return l
}

func newWithTimestamps(logger TestLogger) *Logger {
	l := &Logger{
		logger:    logger,
		startTime: time.Now(),
	}
	l.sprint = l.sprintWithTimestamp
	l.sprintf = l.sprintfWithTimestamp
	l.stopAfterCompletion()
	return l
}

// Stop logging once the test completes; background goroutines may still log.
func (l *Logger) stopAfterCompletion() {
	if cleaner, ok := l.logger.(Cleaner); ok {
		cleaner.Cleanup(func() {
			l.mutex.Lock()
			defer l.mutex.Unlock()
			l.done = true
		})
	}
}

// Log unless the test has completed.
func (l *Logger) emit(message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if !l.done {
		l.logger.Log(message)
	}
}

// Fatal unless the test has completed.
func (l *Logger) emitFatal(message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if !l.done {
		l.logger.Fatal(message)
	}
}

func (l *Logger) sprintWithTimestamp(v ...interface{}) string {
	return fmt.Sprintf("[%09.6fs] %s",
		float64(time.Since(l.startTime))/float64(time.Second),
		plainSprint(v...))
}

func (l *Logger) sprintfWithTimestamp(format string, v ...interface{}) string {
	return fmt.Sprintf("[%09.6fs] %s",
		float64(time.Since(l.startTime))/float64(time.Second),
		plainSprintf(format, v...))
}
