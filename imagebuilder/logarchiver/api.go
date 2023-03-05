package logarchiver

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type BuildInfo struct {
	Error             error
	ImageName         string
	RequestorUsername string
}

type BuildLogArchiver interface {
	AddBuildLog(BuildInfo, []byte) error
}

type BuildLogArchiveOptions struct {
	Quota  uint64
	Topdir string
}

type BuildLogArchiveParams struct {
	Logger log.DebugLogger
}

func New(options BuildLogArchiveOptions,
	params BuildLogArchiveParams) (BuildLogArchiver, error) {
	return newBuildLogArchive(options, params)
}

func NewNullLogger() BuildLogArchiver {
	return newNullLogger()
}
