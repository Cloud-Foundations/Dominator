package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetCommitIdOfRef will return the Commit ID of the specified reference.
func getCommitIdOfRef(topdir, ref string) (string, error) {
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
