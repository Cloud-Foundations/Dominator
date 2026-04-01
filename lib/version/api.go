package version

// Info contains version information for a binary.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Dirty     bool   `json:"dirty"`
}

// Get returns the version information for the current binary.
func Get() Info {
	return get()
}

// String returns a human-readable version string.
func (i Info) String() string {
	return infoString(i)
}
