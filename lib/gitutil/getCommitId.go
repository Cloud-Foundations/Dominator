package gitutil

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

func getCommitIdOfRef(topdir, ref string) (string, error) {
	// First try directly reading.
	if commitId, err := getCommitIdOfRefDirect(topdir, ref); err == nil {
		return commitId, nil
	}
	cmd := exec.Command("git", "log", "--format=format:%H", "-1", ref)
	cmd.Dir = topdir
	_output, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(_output))
	if err != nil {
		if len(output) < 1 {
			return "", fmt.Errorf("error running: git log: %s", err)
		}
		return "", fmt.Errorf("error running: git log: %s: %s", err, output)
	}
	return output, nil
}

func getCommitIdOfRefDirect(topdir, ref string) (string, error) {
	filename := filepath.Join(topdir, ".git", "refs", "heads", ref)
	if lines, err := fsutil.LoadLines(filename); err != nil {
		return "", err
	} else if len(lines) != 1 {
		return "", fmt.Errorf("%s does not have only one line", filename)
	} else {
		return strings.TrimSpace(lines[0]), nil
	}

}
