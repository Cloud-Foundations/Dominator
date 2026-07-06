package version

import (
	_ "embed"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

//go:embed BUILD_INFO
var buildInfoRaw string

type vcsInfo struct {
	revision  string
	buildTime string
	dirty     bool
}

type buildInfo struct {
	version string
	origin  string
	branch  string
	behind  int
	isFork  bool
}

func get() Info {
	vcs := getVCSInfo()
	bi := parseBuildInfo()
	version := bi.version
	if vcs.dirty {
		version += "-dirty"
	}
	return Info{
		Version:       version,
		GitCommit:     vcs.revision,
		GitOrigin:     bi.origin,
		GitBranch:     bi.branch,
		CommitsBehind: bi.behind,
		IsFork:        bi.isFork,
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

func parseBuildInfo() buildInfo {
	info := buildInfo{
		version: "unknown",
		origin:  "unknown",
		branch:  "unknown",
		behind:  -1,
	}
	for _, line := range strings.Split(buildInfoRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "version":
			info.version = val
		case "origin":
			info.origin = val
		case "branch":
			info.branch = val
		case "behind":
			if n, err := strconv.Atoi(val); err == nil {
				info.behind = n
			}
		case "fork":
			info.isFork = val == "true"
		}
	}
	return info
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
