package version

import (
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

// Set at link time by the Makefile with -ldflags -X.
var (
	buildVersion = "unknown"
	buildOrigin  = "unknown"
	buildBranch  = "unknown"
	buildBehind  = "-1"
	buildFork    = "false"
)

type vcsInfo struct {
	revision  string
	buildTime string
	dirty     bool
}

func get() Info {
	vcs := getVCSInfo()
	version := buildVersion
	if vcs.dirty {
		version += "-dirty"
	}
	behind, err := strconv.Atoi(buildBehind)
	if err != nil {
		behind = -1
	}
	return Info{
		Version:       version,
		GitCommit:     vcs.revision,
		GitOrigin:     buildOrigin,
		GitBranch:     buildBranch,
		CommitsBehind: behind,
		IsFork:        buildFork == "true",
		BuildDate:     vcs.buildTime,
		GoVersion:     runtime.Version(),
	}
}

func (i Info) string() string {
	parts := []string{"commit: " + i.GitCommit}
	if i.IsFork {
		parts = append(parts, "origin: "+i.GitOrigin)
	}
	if i.GitBranch != "master" {
		parts = append(parts, "branch: "+i.GitBranch)
	}
	parts = append(parts,
		"behind: "+i.Behind(),
		"built: "+i.BuildDate,
		"go: "+i.GoVersion)
	return i.Version + " (" + strings.Join(parts, ", ") + ")"
}

func (i Info) behind() string {
	switch {
	case i.CommitsBehind < 0:
		return "unknown"
	case i.CommitsBehind == 0:
		return "up to date"
	default:
		return strconv.Itoa(i.CommitsBehind) + " commits"
	}
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
