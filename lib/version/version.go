// Package version provides build version information for all binaries.
//
// Version information is set via ldflags at build time (from git tags).
// Falls back to debug.ReadBuildInfo for dev builds.
package version

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set via ldflags at build time:
//
// go build -ldflags "-X github.com/Cloud-Foundations/Dominator/lib/version.Version=v1.2.3"
var (
	Version   = ""
	GitCommit = ""
	GitBranch = ""
	BuildDate = ""
)

// Info contains version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	GitBranch string `json:"gitBranch"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Dirty     bool   `json:"dirty"`
}

// Get returns version information, with fallbacks for dev builds
func Get() Info {
	// Get VCS info once for commit, dirty, and build time
	vcs := getVCSInfo()

	commit := GitCommit
	dirty := false
	if commit == "" {
		commit = vcs.revision
		dirty = vcs.dirty
	} else {
		dirty = strings.HasSuffix(commit, "-dirty") ||
			strings.HasSuffix(Version, "-dirty")
	}

	branch := GitBranch
	if branch == "" {
		branch = "unknown"
	}

	buildDate := BuildDate
	if buildDate == "" {
		buildDate = vcs.buildTime
	}

	// Build version string
	version := Version
	if version == "" {
		// No ldflags provided - build dev version from VCS info
		version = "dev"
		if vcs.revision != "unknown" {
			version += "+" + vcs.revision
		}
		if vcs.dirty {
			version += "-dirty"
		}
	}

	// Build commit display string (without -dirty suffix, that's in version)
	commitDisplay := commit
	if idx := strings.Index(commitDisplay, "-dirty"); idx != -1 {
		commitDisplay = commitDisplay[:idx]
	}

	return Info{
		Version:   version,
		GitCommit: commitDisplay,
		GitBranch: branch,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		Dirty:     dirty,
	}
}

// String returns a single-line version string
func (i Info) String() string {
	return fmt.Sprintf("%s (commit: %s, branch: %s, built: %s)",
		i.Version, i.GitCommit, i.GitBranch, i.BuildDate)
}

// Short returns just the version number
func (i Info) Short() string {
	return i.Version
}

// Full returns multi-line detailed version info
func (i Info) Full(binaryName string) string {
	return fmt.Sprintf(`%s %s
  Commit: %s
  Branch: %s
  Built:  %s
  Go:     %s`,
		binaryName, i.Version, i.GitCommit,
		i.GitBranch, i.BuildDate, i.GoVersion)
}

// vcsInfo holds version control information from Go build info
type vcsInfo struct {
	revision  string
	buildTime string
	dirty     bool
}

// getVCSInfo extracts all VCS info from Go build info in a single pass
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

// Flag variables for version command-line handling
var (
	showVersion *bool
	showShort   *bool
)

// AddFlags registers -version and -short flags. Call this before flag.Parse().
// Returns a function that should be called after flag.Parse() to handle
// the version flags (prints version and exits if -version was passed).
//
// Usage:
//
//	func main() {
//	    checkVersion := version.AddFlags("myapp")
//	    // ... other flag setup ...
//	    flag.Parse()
//	    checkVersion()
//	    // ... rest of main ...
//	}
func AddFlags(binaryName string) func() {
	showVersion = flag.Bool("version", false, "Print version information and exit")
	showShort = flag.Bool("short", false, "Print short version (use with -version)")

	return func() {
		if *showVersion {
			if *showShort {
				fmt.Println(Get().Short())
			} else {
				fmt.Println(Get().Full(binaryName))
			}
			os.Exit(0)
		}
	}
}
