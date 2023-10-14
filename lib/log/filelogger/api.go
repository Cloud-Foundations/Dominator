package filelogger

import (
	"bufio"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
)

type Logger struct {
	*debuglogger.Logger
	file   *os.File
	writer *bufio.Writer
}

type Options struct {
	Flags      int
	DebugLevel int16 // Supported range: -1 to 32767.
}

// New will create a *Logger with the specified filename and options.
func New(filename string, options Options) (*Logger, error) {
	return newLogger(filename, options)
}

func (l *Logger) Close() error {
	return l.close()
}

func (l *Logger) Flush() error {
	return l.writer.Flush()
}
