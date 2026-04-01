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

type vcsInfo struct {
	revision  string
	buildTime string
	dirty     bool
}

func get() Info {
	vcs := getVCSInfo()
	version := strings.TrimSpace(baseVersion)
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

func infoString(i Info) string {
	return fmt.Sprintf("%s (built: %s)", i.Version, i.BuildDate)
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
