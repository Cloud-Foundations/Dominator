package filesystem

import (
	"fmt"
	"io"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

const (
	timeFormat  string = "02 Jan 2006 15:04:05 MST"
	symlinkMode        = wsyscall.S_IFLNK | wsyscall.S_IRWXU |
		wsyscall.S_IRWXG | wsyscall.S_IRWXO
)

func (fs *FileSystem) list(w io.Writer, listSelector ListSelector,
	filter *filter.Filter) error {
	numLinksTable := buildNumLinksTable(fs)
	return fs.DirectoryInode.list(w, "/", numLinksTable, 1, listSelector,
		filter)
}

func buildNumLinksTable(fs *FileSystem) NumLinksTable {
	numLinksTable := make(NumLinksTable)
	fs.DirectoryInode.scanDirectory(fs, numLinksTable)
	return numLinksTable
}

func (inode *DirectoryInode) scanDirectory(fs *FileSystem,
	numLinksTable NumLinksTable) {
	for _, dirent := range inode.EntryList {
		numLinksTable[dirent.InodeNumber]++
		if inode, ok := dirent.Inode().(*DirectoryInode); ok {
			inode.scanDirectory(fs, numLinksTable)
		}
	}
}

func (inode *DirectoryInode) list(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	if err := listUntilName(w, inode.Mode, numLinks, inode.Uid, inode.Gid,
		0, -1, -1, name, true, listSelector); err != nil {
		return err
	}
	for _, dirent := range inode.EntryList {
		pathname := path.Join(name, dirent.Name)
		if filter != nil && filter.Match(pathname) {
			continue
		}
		err := dirent.inode.List(w, pathname, numLinksTable,
			numLinksTable[dirent.InodeNumber], listSelector, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func (inode *RegularInode) list(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector) error {
	if err := listUntilName(w, inode.Mode, numLinks, inode.Uid, inode.Gid,
		inode.Size, inode.MtimeSeconds, inode.MtimeNanoSeconds, name, false,
		listSelector); err != nil {
		return err
	}
	var err error
	if inode.Size > 0 && listSelector&ListSelectSkipData == 0 {
		_, err = fmt.Fprintf(w, " %x\n", inode.Hash)
	} else {
		_, err = io.WriteString(w, "\n")
	}
	return err
}

func (inode *ComputedRegularInode) list(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector) error {
	if err := listUntilName(w, inode.Mode, numLinks, inode.Uid, inode.Gid,
		0, -1, -1, name, false, listSelector); err != nil {
		return err
	}
	var err error
	if listSelector&ListSelectSkipData == 0 {
		_, err = fmt.Fprintf(w, " <- %s\n", inode.Source)
	} else {
		_, err = io.WriteString(w, "\n")
	}
	return err
}

func (inode *SymlinkInode) list(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector) error {
	if err := listUntilName(w, symlinkMode, numLinks, inode.Uid, inode.Gid,
		0, -1, -1, name, false, listSelector); err != nil {
		return err
	}
	var err error
	if listSelector&ListSelectSkipData == 0 {
		_, err = fmt.Fprintf(w, " -> %s\n", inode.Symlink)
	} else {
		_, err = io.WriteString(w, "\n")
	}
	return err
}

func (inode *SpecialInode) list(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector) error {
	return listUntilName(w, inode.Mode, numLinks, inode.Uid, inode.Gid,
		inode.Rdev, inode.MtimeSeconds, inode.MtimeNanoSeconds, name, true,
		listSelector)
}

func listUntilName(w io.Writer, mode FileMode, numLinks int, uid uint32,
	gid uint32, data uint64, seconds int64, nanoSeconds int32, name string,
	newline bool, listSelector ListSelector) error {
	if listSelector&ListSelectSkipMode == 0 {
		if _, err := io.WriteString(w, mode.String()+" "); err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipNumLinks == 0 {
		if _, err := fmt.Fprintf(w, "%3d ", numLinks); err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipUid == 0 {
		if _, err := fmt.Fprintf(w, "%5d ", uid); err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipGid == 0 {
		if _, err := fmt.Fprintf(w, "%5d ", gid); err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipSizeDevnum == 0 {
		var err error
		switch mode & wsyscall.S_IFMT {
		case wsyscall.S_IFREG:
			if data == 0 && seconds < 0 && nanoSeconds < 0 {
				_, err = fmt.Fprintf(w, "%10s ", "computed")
			} else {
				_, err = fmt.Fprintf(w, "%10d ", data)
			}
		case wsyscall.S_IFBLK, wsyscall.S_IFCHR:
			_, err = fmt.Fprintf(w, "%#10x ", data)
		default:
			_, err = fmt.Fprintf(w, "%11s", "")
		}
		if err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipMtime == 0 {
		var err error
		if seconds == -1 && nanoSeconds == -1 {
			_, err = fmt.Fprintf(w, "%25s", "")
		} else {
			t := time.Unix(seconds, int64(nanoSeconds))
			_, err = io.WriteString(w, t.Format(timeFormat)+" ")
		}
		if err != nil {
			return err
		}
	}
	if listSelector&ListSelectSkipName == 0 {
		if _, err := io.WriteString(w, name); err != nil {
			return err
		}
	}
	if newline {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}

func (mode FileMode) string() string {
	var buf [10]byte
	w := 1
	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if mode&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	switch mode & wsyscall.S_IFMT {
	case wsyscall.S_IFSOCK:
		buf[0] = 's'
	case wsyscall.S_IFLNK:
		buf[0] = 'l'
	case wsyscall.S_IFREG:
		buf[0] = '-'
	case wsyscall.S_IFBLK:
		buf[0] = 'b'
	case wsyscall.S_IFDIR:
		buf[0] = 'd'
	case wsyscall.S_IFCHR:
		buf[0] = 'c'
	case wsyscall.S_IFIFO:
		buf[0] = 'p'
	default:
		buf[0] = '?'
	}
	if mode&wsyscall.S_ISUID != 0 {
		if mode&wsyscall.S_IXUSR == 0 {
			buf[3] = 'S'
		} else {
			buf[3] = 's'
		}
	}
	if mode&wsyscall.S_ISGID != 0 {
		if mode&wsyscall.S_IXGRP == 0 {
			buf[6] = 'S'
		} else {
			buf[6] = 's'
		}
	}
	if mode&wsyscall.S_ISVTX != 0 {
		if mode&wsyscall.S_IXOTH == 0 {
			buf[9] = 'T'
		} else {
			buf[9] = 't'
		}
	}
	return string(buf[:])
}
