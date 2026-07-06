package version

// Info contains version information for a binary.
type Info struct {
	Version       string
	GitCommit     string
	GitOrigin     string
	GitBranch     string
	CommitsBehind int
	IsFork        bool
	BuildDate     string
	GoVersion     string
}

// Get returns the version information for the current binary.
func Get() Info {
	return get()
}

// String returns a single-line human-readable version string.
func (i Info) String() string {
	return i.string()
}

// Behind returns a human-readable description of how far behind upstream
// the build is: "unknown" if the check was not run, "up to date" if on the
// upstream tip, or "N commits" otherwise.
func (i Info) Behind() string {
	return i.behind()
}
