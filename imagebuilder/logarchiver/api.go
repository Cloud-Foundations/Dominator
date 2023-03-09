package logarchiver

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type BuildInfo struct {
	Duration          time.Duration `json:",omitempty"`
	Error             string        `json:",omitempty"`
	RequestorUsername string        `json:",omitempty"`
}

type BuildLogArchiver interface {
	AddBuildLog(string, BuildInfo, []byte) error
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
