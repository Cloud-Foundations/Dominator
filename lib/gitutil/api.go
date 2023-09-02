package gitutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type ShallowCloneParams struct {
	GitBranch string   // Branch to fetch.
	Patterns  []string // Patterns to fetch.
	PublicURL string   // Repository URL which is safe to log.
	RepoURL   string   // Real URL of repository.
}

// GetCommitIdOfRef will return the Commit ID of the specified reference.
func GetCommitIdOfRef(topdir, ref string) (string, error) {
	return getCommitIdOfRef(topdir, ref)
}

// ShallowClone will make a shallow clone of a Git repository. The repository
// will be written to the directory specified by topdir.
func ShallowClone(topdir string, params ShallowCloneParams,
	logger log.DebugLogger) error {
	return shallowClone(topdir, params, logger)
}
