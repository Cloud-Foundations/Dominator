package filesystem

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

var modePerm FileMode = wsyscall.S_IRWXU | wsyscall.S_IRWXG | wsyscall.S_IRWXO

func forceWriteMetadata(inode GenericInode, name string) error {
	err := inode.WriteMetadata(name)
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		// Blindly attempt to remove immutable attributes.
		fsutil.MakeMutable(name)
	}
	return inode.WriteMetadata(name)
}

func (inode *DirectoryInode) write(name string) error {
	if err := inode.make(name); err != nil {
		// If existing directory, don't blow it away, just update metadata.
		if os.IsExist(err) {
			if fi, err := os.Lstat(name); err == nil && fi.IsDir() {
				return inode.writeMetadata(name)
			}
		}
		fsutil.ForceRemoveAll(name)
		if err := inode.make(name); err != nil {
			return err
		}
	}
	return inode.writeMetadata(name)
}

func (inode *DirectoryInode) make(name string) error {
	return syscall.Mkdir(name, uint32(inode.Mode))
}

func (inode *DirectoryInode) writeMetadata(name string) error {
	if err := os.Lchown(name, int(inode.Uid), int(inode.Gid)); err != nil {
		return err
	}
	return syscall.Chmod(name, uint32(inode.Mode))
}

func (inode *RegularInode) writeMetadata(name string) error {
	if err := os.Lchown(name, int(inode.Uid), int(inode.Gid)); err != nil {
		return err
	}
	if err := syscall.Chmod(name, uint32(inode.Mode)); err != nil {
		return err
	}
	t := time.Unix(inode.MtimeSeconds, int64(inode.MtimeNanoSeconds))
	return os.Chtimes(name, t, t)
}

func (inode *SymlinkInode) write(name string) error {
	if inode.make(name) != nil {
		fsutil.ForceRemoveAll(name)
		if err := inode.make(name); err != nil {
			return err
		}
	}
	return inode.writeMetadata(name)
}

func (inode *SymlinkInode) make(name string) error {
	return os.Symlink(inode.Symlink, name)
}

func (inode *SymlinkInode) writeMetadata(name string) error {
	return os.Lchown(name, int(inode.Uid), int(inode.Gid))
}

func (inode *SpecialInode) write(name string) error {
	if inode.make(name) != nil {
		fsutil.ForceRemoveAll(name)
		if err := inode.make(name); err != nil {
			return err
		}
	}
	return inode.writeMetadata(name)
}

func (inode *SpecialInode) make(name string) error {
	if inode.Mode&syscall.S_IFBLK != 0 || inode.Mode&syscall.S_IFCHR != 0 {
		return wsyscall.Mknod(name, uint32(inode.Mode), int(inode.Rdev))
	} else if inode.Mode&syscall.S_IFIFO != 0 {
		return wsyscall.Mkfifo(name, uint32(inode.Mode))
	} else {
		return errors.New("unsupported mode")
	}
}

func (inode *SpecialInode) writeMetadata(name string) error {
	if err := os.Lchown(name, int(inode.Uid), int(inode.Gid)); err != nil {
		return err
	}
	if err := syscall.Chmod(name, uint32(inode.Mode)); err != nil {
		return err
	}
	t := time.Unix(inode.MtimeSeconds, int64(inode.MtimeNanoSeconds))
	return os.Chtimes(name, t, t)
}
