package logarchiver

import (
	"io"
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

type BuildLogReporter interface {
	GetBuildInfosForRequestor(username string, incGood, incBad bool) *BuildInfos
	GetBuildInfosForStream(streamName string, incGood, incBad bool) *BuildInfos
	GetBuildLog(imageName string) (io.ReadCloser, error)
	GetSummary() *Summary
}

type BuildLogger interface {
	BuildLogArchiver
	BuildLogReporter
}

type BuildInfos struct {
	Builds map[string]BuildInfo // Key: image name.
}

type RequestorSummary struct {
	NumBuilds      uint64
	NumGoodBuilds  uint64
	NumErrorBuilds uint64
}

type StreamSummary struct {
	NumBuilds      uint64
	NumGoodBuilds  uint64
	NumErrorBuilds uint64
}

type Summary struct {
	Requestors map[string]*RequestorSummary // Key: username.
	Streams    map[string]*StreamSummary    // Key: stream name.
}

func New(options BuildLogArchiveOptions,
	params BuildLogArchiveParams) (BuildLogger, error) {
	return newBuildLogArchive(options, params)
}

func NewNullLogger() BuildLogArchiver {
	return newNullLogger()
}
