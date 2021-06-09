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

type GetDirectedGraphRequest struct {
	MaxAge time.Duration
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
