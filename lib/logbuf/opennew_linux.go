package logbuf

import (
	"os"
	"path"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/bufwriter"
)

const (
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
	filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP |
		syscall.S_IROTH
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
		syscall.Dup3(int(file.Fd()), int(os.Stdout.Fd()), 0)
		syscall.Dup3(int(file.Fd()), int(os.Stderr.Fd()), 0)
	}
	lb.file = file
	lb.writer = bufwriter.NewWriter(file, time.Second)
	symlink := path.Join(lb.options.Directory, "latest")
	tmpSymlink := symlink + "~"
	os.Remove(tmpSymlink)
	os.Symlink(filename, tmpSymlink)
	return os.Rename(tmpSymlink, symlink)
}
