package untar

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/build"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
)

type decoderData struct {
	builder        *build.Builder
	directoryTable map[string]*filesystem.DirectoryInode // Key: full name.
	inodeTable     map[string]filesystem.GenericInode    // Key: full name.
}

func normaliseFilename(filename string) string {
	if filename[:2] == "./" {
		filename = filename[1:]
	} else if filename[0] != '/' {
		filename = "/" + filename
	}
	length := len(filename)
	if length > 1 && filename[length-1] == '/' {
		filename = filename[:length-1]
	}
	return filename
}

func decode(tarReader *tar.Reader, hasher Hasher, filter *filter.Filter) (
	*filesystem.FileSystem, error) {
	dd := decoderData{
		builder:        build.New(),
		directoryTable: make(map[string]*filesystem.DirectoryInode),
		inodeTable:     make(map[string]filesystem.GenericInode),
	}
	dd.directoryTable["/"] = &dd.builder.FileSystem.DirectoryInode
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		header.Name = normaliseFilename(header.Name)
		if header.Name == "/.subd" ||
			strings.HasPrefix(header.Name, "/.subd/") {
			continue
		}
		if filter != nil && filter.Match(header.Name) {
			continue
		}
		err = dd.addHeader(tarReader, hasher, header)
		if err != nil {
			return nil, err
		}
	}
	dd.builder.Sort()
	return dd.builder.FileSystem, nil
}

func (dd *decoderData) addDirectory(header *tar.Header,
	parent *filesystem.DirectoryInode, leafName string) error {
	newInode := filesystem.DirectoryInode{
		Mode: filesystem.FileMode((header.Mode & ^syscall.S_IFMT) |
			syscall.S_IFDIR),
		Uid: uint32(header.Uid),
		Gid: uint32(header.Gid),
	}
	if header.Name == "/" {
		*dd.directoryTable[header.Name] = newInode
		return nil
	}
	dd.directoryTable[header.Name] = &newInode
	return dd.addInodeAndEntry(parent, header.Name, leafName, &newInode)
}

func (dd *decoderData) addInodeAndEntry(parent *filesystem.DirectoryInode,
	fullName, leafName string, inode filesystem.GenericInode) error {
	if _, ok := dd.inodeTable[fullName]; ok {
		return fmt.Errorf("%s already added", fullName)
	}
	dd.inodeTable[fullName] = inode
	if err := dd.builder.AddInode(inode); err != nil {
		return err
	}
	return dd.builder.AddDirectoryEntry(parent, leafName, inode)
}

func (dd *decoderData) addHeader(tarReader *tar.Reader, hasher Hasher,
	header *tar.Header) error {
	parentDir, ok := dd.directoryTable[path.Dir(header.Name)]
	if !ok {
		return fmt.Errorf("no parent directory found for: %s", header.Name)
	}
	leafName := path.Base(header.Name)
	if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
		return dd.addRegularFile(tarReader, hasher, header,
			parentDir, leafName)
	} else if header.Typeflag == tar.TypeLink {
		return dd.addHardlink(header, parentDir, leafName)
	} else if header.Typeflag == tar.TypeSymlink {
		return dd.addSymlink(header, parentDir, leafName)
	} else if header.Typeflag == tar.TypeChar {
		return dd.addSpecialFile(header, parentDir, leafName)
	} else if header.Typeflag == tar.TypeBlock {
		return dd.addSpecialFile(header, parentDir, leafName)
	} else if header.Typeflag == tar.TypeDir {
		return dd.addDirectory(header, parentDir, leafName)
	} else if header.Typeflag == tar.TypeFifo {
		return dd.addSpecialFile(header, parentDir, leafName)
	} else {
		return fmt.Errorf("unsupported file type: %v", header.Typeflag)
	}
}

func (dd *decoderData) addRegularFile(tarReader *tar.Reader,
	hasher Hasher, header *tar.Header, parent *filesystem.DirectoryInode,
	name string) error {
	newInode := filesystem.RegularInode{
		Mode: filesystem.FileMode((header.Mode & ^syscall.S_IFMT) |
			syscall.S_IFREG),
		Uid:              uint32(header.Uid),
		Gid:              uint32(header.Gid),
		MtimeNanoSeconds: int32(header.ModTime.Nanosecond()),
		MtimeSeconds:     header.ModTime.Unix(),
		Size:             uint64(header.Size)}
	if header.Size > 0 {
		var err error
		newInode.Hash, err = hasher.Hash(tarReader, uint64(header.Size))
		if err != nil {
			return err
		}
	}
	return dd.addInodeAndEntry(parent, header.Name, name, &newInode)
}

func (dd *decoderData) addHardlink(header *tar.Header,
	parent *filesystem.DirectoryInode, leafName string) error {
	header.Linkname = normaliseFilename(header.Linkname)
	if inode, ok := dd.inodeTable[header.Linkname]; !ok {
		return fmt.Errorf("missing hardlink target: %s", header.Linkname)
	} else {
		return dd.builder.AddDirectoryEntry(parent, leafName, inode)
	}
}

func (dd *decoderData) addSpecialFile(header *tar.Header,
	parent *filesystem.DirectoryInode, name string) error {
	var newInode filesystem.SpecialInode
	if header.Typeflag == tar.TypeChar {
		newInode.Mode = filesystem.FileMode((header.Mode & ^syscall.S_IFMT) |
			syscall.S_IFCHR)
	} else if header.Typeflag == tar.TypeBlock {
		newInode.Mode = filesystem.FileMode((header.Mode & ^syscall.S_IFMT) |
			syscall.S_IFBLK)
	} else if header.Typeflag == tar.TypeFifo {
		newInode.Mode = filesystem.FileMode((header.Mode & ^syscall.S_IFMT) |
			syscall.S_IFIFO)
	} else {
		return fmt.Errorf("unsupported type: %v", header.Typeflag)
	}
	newInode.Uid = uint32(header.Uid)
	newInode.Gid = uint32(header.Gid)
	newInode.MtimeNanoSeconds = int32(header.ModTime.Nanosecond())
	newInode.MtimeSeconds = header.ModTime.Unix()
	if header.Devminor > 255 {
		return fmt.Errorf("minor device number: %d too large",
			header.Devminor)
	}
	newInode.Rdev = uint64(header.Devmajor<<8 | header.Devminor)
	return dd.addInodeAndEntry(parent, header.Name, name, &newInode)
}

func (dd *decoderData) addSymlink(header *tar.Header,
	parent *filesystem.DirectoryInode, name string) error {
	newInode := filesystem.SymlinkInode{
		Uid:     uint32(header.Uid),
		Gid:     uint32(header.Gid),
		Symlink: header.Linkname}
	return dd.addInodeAndEntry(parent, header.Name, name, &newInode)
}
