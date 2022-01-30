package rpcd

import (
	"os/exec"
	"strings"
)

func (t *rpcType) generateNote() (string, error) {
	if t.config.NoteGeneratorCommand == "" {
		return "", nil
	}
	cmd := exec.Command(t.config.NoteGeneratorCommand)
	if output, err := cmd.Output(); err != nil {
		return "", err
	} else if length := len(output); length > 0 {
		var note string
		if length < 64 {
			note = string(output)
		} else {
			note = string(output[:64])
		}
		return strings.TrimSpace(note), nil
	}
	return "", nil
}
