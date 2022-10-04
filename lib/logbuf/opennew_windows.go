package logbuf

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/bufwriter"
)

const (
	dirPerms  = os.ModeDir | os.ModePerm
	filePerms = os.ModePerm
)

// This should be called with the lock held.
func (lb *LogBuffer) openNewFile() error {
	lb.fileSize = 0
	filename := time.Now().Format(timeLayout)
	file, err := os.OpenFile(path.Join(lb.options.Directory, filename),
		os.O_CREATE|os.O_WRONLY, filePerms)
	if err != nil {
		return err
	}
	if lb.options.RedirectStderr {
		return fmt.Errorf("redirectot to stderr not supported on windows")
	}
	lb.file = file
	lb.writer = bufwriter.NewWriter(file, time.Second)
	symlink := path.Join(lb.options.Directory, "latest")
	tmpSymlink := symlink + "~"
	os.Remove(tmpSymlink)
	os.Symlink(filename, tmpSymlink)
	return os.Rename(tmpSymlink, symlink)
}
