// +build !linux

package fsutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func watchFileWithFsNotify(pathname string, logger log.Logger) <-chan struct{} {
	return nil
}

func watchFileStopWithFsNotify() bool { return false }
