package gitutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getTreeSize(dirname string) (uint64, error) {
	var size uint64
	err := filepath.Walk(dirname,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			size += uint64(info.Size())
			return nil
		})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func runCommand(logger log.DebugLogger, cwd string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	if writer, ok := logger.(io.Writer); ok {
		cmd.Stdout = writer
		cmd.Stderr = writer
		return cmd.Run()
	}
	if _output, err := cmd.CombinedOutput(); err != nil {
		output := strings.TrimSpace(string(_output))
		if len(output) < 1 {
			return fmt.Errorf("error running: %s: %s", args[0], err)
		}
		return fmt.Errorf("error running: %s: %s: %s", args[0], err, output)
	}
	return nil
}

func shallowClone(topdir string, params ShallowCloneParams,
	logger log.DebugLogger) error {
	if params.PublicURL == "" {
		params.PublicURL = params.RepoURL
	}
	if params.GitBranch != "" {
		logger.Debugf(0, "Cloning repository: %s branch: %s\n",
			params.PublicURL, params.GitBranch)
	} else {
		logger.Debugf(0, "Cloning repository: %s\n", params.PublicURL)
	}
	startTime := time.Now()
	err := runCommand(logger, "", "git", "init", topdir)
	if err != nil {
		return err
	}
	err = runCommand(logger, topdir, "git", "remote", "add", "origin",
		params.RepoURL)
	if err != nil {
		return err
	}
	if len(params.Patterns) > 0 {
		err := runCommand(logger, topdir, "git", "config",
			"core.sparsecheckout", "true")
		if err != nil {
			return err
		}
		file, err := os.Create(
			filepath.Join(topdir, ".git", "info", "sparse-checkout"))
		if err != nil {
			return err
		}
		defer file.Close()
		writer := bufio.NewWriter(file)
		defer writer.Flush()
		for _, pattern := range params.Patterns {
			fmt.Fprintln(writer, pattern)
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	err = runCommand(logger, topdir, "git", "pull", "--depth=1",
		"origin", params.GitBranch)
	if err != nil {
		return err
	}
	if params.GitBranch != "" {
		err = runCommand(logger, topdir, "git", "checkout", params.GitBranch)
		if err != nil {
			return err
		}
	}
	loadTime := time.Since(startTime)
	repoSize, err := getTreeSize(topdir)
	if err != nil {
		return err
	}
	speed := float64(repoSize) / loadTime.Seconds()
	logger.Debugf(0,
		"Downloaded partial repository in %s, size: %s (%s/s)\n",
		format.Duration(loadTime), format.FormatBytes(repoSize),
		format.FormatBytes(uint64(speed)))
	return nil
}
