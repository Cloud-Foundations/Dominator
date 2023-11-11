package filelogger

import (
	"bufio"
	"log"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
)

func newLogger(filename string, options Options) (*Logger, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	writer := bufio.NewWriter(file)
	return &Logger{
		Logger: debuglogger.New(log.New(writer, "", options.Flags)),
		file:   file,
		writer: writer,
	}, nil
}

func (l *Logger) close() error {
	flushError := l.Flush()
	if err := l.file.Close(); err != nil {
		return err
	}
	return flushError
}
