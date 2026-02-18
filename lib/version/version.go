package version

import (
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

//go:embed VERSION
var baseVersion string

type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Dirty     bool   `json:"dirty"`
}

func Get() Info {
	vcs := getVCSInfo()
	version := strings.TrimSpace(baseVersion)
	if vcs.revision != "unknown" {
		version += "+" + vcs.revision
	}
	if vcs.dirty {
		version += "-dirty"
	}
	return Info{
		Version:   version,
		GitCommit: vcs.revision,
		BuildDate: vcs.buildTime,
		GoVersion: runtime.Version(),
		Dirty:     vcs.dirty,
	}
}

func (i Info) String() string {
	return fmt.Sprintf("%s (built: %s)", i.Version, i.BuildDate)
}

type vcsInfo struct {
	revision  string
	buildTime string
	dirty     bool
}

func getVCSInfo() vcsInfo {
	info := vcsInfo{
		revision:  "unknown",
		buildTime: "unknown",
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}

	for _, s := range buildInfo.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 8 {
				info.revision = s.Value[:8]
			} else {
				info.revision = s.Value
			}
		case "vcs.time":
			info.buildTime = s.Value
		case "vcs.modified":
			info.dirty = s.Value == "true"
		}
	}

	return info
}
