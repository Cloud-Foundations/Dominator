package builder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func gitShallowClone(manifestRoot, manifestUrl, gitBranch string,
	patterns []string, buildLog io.Writer) error {
	fmt.Fprintf(buildLog, "Cloning repository: %s branch: %s\n",
		manifestUrl, gitBranch)
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
	return nil
}
