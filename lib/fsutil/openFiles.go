package fsutil

import (
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
)

type openFile struct {
	file  *os.File
	error error
}

func openFiles(g *goroutine.Goroutine, pathnames []string,
	queueSize uint) *FilesOpener {
	cancelChannel := make(chan struct{}, 1)
	resultsChannel := make(chan openFile, queueSize)
	fo := &FilesOpener{
		cancel:    cancelChannel,
		pathnames: pathnames,
		results:   resultsChannel,
	}
	if g == nil {
		go fo.manage(cancelChannel, resultsChannel)
	} else {
		g.Run(func() { fo.manage(cancelChannel, resultsChannel) })
	}
	return fo
}

func (fo *FilesOpener) manage(cancelChannel <-chan struct{},
	resultsChannel chan openFile) {
	var cancelled bool
	for _, pathname := range fo.pathnames {
		file, err := os.Open(pathname)
		select {
		case resultsChannel <- openFile{file, err}:
		case <-cancelChannel:
			cancelled = true
		}
		if cancelled {
			break
		}
	}
	if cancelled {
		for result := range resultsChannel {
			if result.file != nil {
				result.file.Close()
			}
		}
	}
}

func (fo *FilesOpener) openNext() (*os.File, error) {
	result := <-fo.results
	return result.file, result.error
}
