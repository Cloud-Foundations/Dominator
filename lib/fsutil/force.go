package fsutil

import (
	"os"
	"syscall"
)

func forceLink(oldname, newname string) error {
	err := os.Link(oldname, newname)
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		// Blindly attempt to remove immutable attributes.
		MakeMutable(oldname, newname)
	} else if os.IsExist(err) {
		forceRemove(newname)
	}
	return os.Link(oldname, newname)
}

func forceRemove(name string) error {
	err := os.Remove(name)
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		// Blindly attempt to remove immutable attributes.
		MakeMutable(name)
	}
	return os.Remove(name)
}

func forceRemoveAll(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		// Blindly attempt to remove immutable attributes.
		MakeMutable(path)
	}
	return os.RemoveAll(path)
}

func forceRename(oldpath, newpath string) error {
	err := os.Rename(oldpath, newpath)
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		// Blindly attempt to remove immutable attributes.
		MakeMutable(oldpath, newpath)
	} else if os.IsExist(err) {
		if err := ForceRemoveAll(newpath); err != nil {
			return err
		}
	} else if err.(*os.LinkError).Err == syscall.EISDIR {
		if err := ForceRemoveAll(newpath); err != nil {
			return err
		}
	}
	return os.Rename(oldpath, newpath)
}
