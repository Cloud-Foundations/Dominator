package manager

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

const (
	qemuVersionMatchString = "QEMU emulator version"
)

type qemuVersionType struct {
	major    uint
	minor    uint
	subminor uint
}

var (
	qemuVersionMutex sync.Mutex
	lastQemuVersion  qemuVersionType
)

func getQemuVersion(logger log.Logger) (qemuVersionType, error) {
	cmd := exec.Command(*qemuCommand, "-version")
	stdout, err := cmd.Output()
	if err != nil {
		return qemuVersionType{}, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		if !strings.HasPrefix(line, qemuVersionMatchString) {
			continue
		}
		var info qemuVersionType
		_, err := fmt.Sscanf(line[len(qemuVersionMatchString):], " %d.%d.%d ",
			&info.major, &info.minor, &info.subminor)
		if err != nil {
			return qemuVersionType{}, err
		}
		qemuVersionMutex.Lock()
		defer qemuVersionMutex.Unlock()
		if info != lastQemuVersion {
			logger.Printf("Detected QEMU version: %d.%d.%d\n",
				info.major, info.minor, info.subminor)
			lastQemuVersion = info
		}
		return info, nil
	}
	if err := scanner.Err(); err != nil {
		return qemuVersionType{}, err
	}
	return qemuVersionType{}, fmt.Errorf("no QEMU version info found")
}
