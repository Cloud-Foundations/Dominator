package builder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

// gitShallowClone will make a shallow clone of a Git repository. The repository
// will be written to the directory specified by manifestRoot. The URL of the
// repository must be specified by manifestUrl and the public version (not
// containing secrets) must be specified by publicUrl. The branch to fetch must
// be specified by gitBranch. Only the patterns specified will be fetched.
// The fetch log fill be written to buildLog.
func gitShallowClone(manifestRoot, manifestUrl, publicUrl, gitBranch string,
	patterns []string, buildLog io.Writer) error {
	fmt.Fprintf(buildLog, "Cloning repository: %s branch: %s\n",
		publicUrl, gitBranch)
	startTime := time.Now()
	err := runCommand(buildLog, "", "git", "init", manifestRoot)
	if err != nil {
		return err
	}
	err = runCommand(buildLog, manifestRoot, "git", "remote", "add", "origin",
		manifestUrl)
	if err != nil {
		return err
	}
	if len(patterns) > 0 {
		err := runCommand(buildLog, manifestRoot, "git", "config",
			"core.sparsecheckout", "true")
		if err != nil {
			return err
		}
		file, err := os.Create(
			filepath.Join(manifestRoot, ".git", "info", "sparse-checkout"))
		if err != nil {
			return err
		}
		defer file.Close()
		writer := bufio.NewWriter(file)
		defer writer.Flush()
		for _, pattern := range patterns {
			fmt.Fprintln(writer, pattern)
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	err = runCommand(buildLog, manifestRoot, "git", "pull", "--depth=1",
		"origin", gitBranch)
	if err != nil {
		return err
	}
	if gitBranch != "master" {
		err = runCommand(buildLog, manifestRoot, "git", "checkout", gitBranch)
		if err != nil {
			return err
		}
	}
	loadTime := time.Since(startTime)
	repoSize, err := getTreeSize(manifestRoot)
	if err != nil {
		return err
	}
	speed := float64(repoSize) / loadTime.Seconds()
	fmt.Fprintf(buildLog,
		"Downloaded partial repository in %s, size: %s (%s/s)\n",
		format.Duration(loadTime), format.FormatBytes(repoSize),
		format.FormatBytes(uint64(speed)))
	return nil
}
