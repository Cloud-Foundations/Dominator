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
	return &Logger{
		logger:  logger,
		sprint:  plainSprint,
		sprintf: plainSprintf,
	}
}

func newWithTimestamps(logger TestLogger) *Logger {
	l := &Logger{
		logger:    logger,
		startTime: time.Now(),
	}
	l.sprint = l.sprintWithTimestamp
	l.sprintf = l.sprintfWithTimestamp
	return l
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
