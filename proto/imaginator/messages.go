package imaginator

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/image"
)

type BuildImageRequest struct {
	DisableRecursiveBuild bool
	ExpiresIn             time.Duration
	GitBranch             string
	MaxSourceAge          time.Duration
	ReturnImage           bool
	StreamBuildLog        bool
	StreamName            string
	Variables             map[string]string
}

type BuildImageResponse struct {
	Image       *image.Image
	ImageName   string
	BuildLog    []byte
	ErrorString string
}

type DisableAutoBuildsRequest struct {
	DisableFor time.Duration
}

type DisableAutoBuildsResponse struct {
	DisabledUntil time.Time
	Error         string
}

type DisableBuildRequestsRequest struct {
	DisableFor time.Duration
}

type DisableBuildRequestsResponse struct {
	DisabledUntil time.Time
	Error         string
}

type GetDependenciesRequest struct {
	MaxAge time.Duration
}

type GetDependenciesResponse struct {
	GetDependenciesResult
	Error string
}

type GetDependenciesResult struct {
	FetchLog           []byte
	GeneratedAt        time.Time
	LastAttemptAt      time.Time
	LastAttemptError   string
	StreamToSource     map[string]string // K: stream name, V: source stream.
	UnbuildableSources map[string]struct{}
}

type GetDirectedGraphRequest struct {
	Excludes []string
	Includes []string
	MaxAge   time.Duration
}

type GetDirectedGraphResponse struct {
	GetDirectedGraphResult
	Error string
}

type GetDirectedGraphResult struct {
	FetchLog         []byte
	GeneratedAt      time.Time
	GraphvizDot      []byte
	LastAttemptAt    time.Time
	LastAttemptError string
}

type ReplaceIdleSlavesRequest struct {
	ImmediateGetNew bool
}

type ReplaceIdleSlavesResponse struct {
	Error string
}
