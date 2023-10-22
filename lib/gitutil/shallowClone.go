package gitutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func gitGrab(topdir string, gitBranch string, logger log.DebugLogger) error {
	if gitBranch == "" {
		return runCommand(logger, topdir, "git", "pull", "--depth=1", "origin",
			"")
	}
	err := runCommand(logger, topdir, "git", "fetch", "--depth=1", "origin")
	if err != nil {
		return err
	}
	branchList, err := fsutil.ReadDirnames(filepath.Join(topdir, ".git", "refs",
		"remotes", "origin"), false)
	if err != nil {
		return err
	}
	branchMap := stringutil.ConvertListToMap(branchList, false)
	var branchToMerge string
	if _, ok := branchMap[gitBranch]; ok {
		branchToMerge = gitBranch
	} else {
		branchToMerge = branchList[0]
	}
	err = runCommand(logger, topdir, "git", "merge",
		path.Join("origin", branchToMerge))
	if err != nil {
		return err
	}
	if gitBranch != "" && branchToMerge != gitBranch {
		err := runCommand(logger, topdir, "git", "fetch", "--unshallow",
			"origin")
		if err != nil {
			return err
		}
		err = runCommand(logger, topdir, "git", "checkout", gitBranch)
		if err != nil {
			return err
		}
	}
	return nil
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
			return fmt.Errorf("error running: %s %s: %s", args[0], args[1], err)
		}
		return fmt.Errorf("error running: %s %s: %s: %s",
			args[0], args[1], err, output)
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
	err := runCommand(logger, "", "git", "init", "-b", "master", topdir)
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
	if err := gitGrab(topdir, params.GitBranch, logger); err != nil {
		return err
	}
	loadTime := time.Since(startTime)
	repoSize, err := fsutil.GetTreeSize(topdir)
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
