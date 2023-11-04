package scanner

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/fsrateio"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func makeRegularInode(stat *wsyscall.Stat_t) *filesystem.RegularInode {
	var inode filesystem.RegularInode
	inode.Mode = filesystem.FileMode(stat.Mode)
	inode.Uid = stat.Uid
	inode.Gid = stat.Gid
	inode.MtimeSeconds = int64(stat.Mtim.Sec)
	inode.MtimeNanoSeconds = int32(stat.Mtim.Nsec)
	inode.Size = uint64(stat.Size)
	return &inode
}

func makeSymlinkInode(stat *wsyscall.Stat_t) *filesystem.SymlinkInode {
	var inode filesystem.SymlinkInode
	inode.Uid = stat.Uid
	inode.Gid = stat.Gid
	return &inode
}

func makeSpecialInode(stat *wsyscall.Stat_t) *filesystem.SpecialInode {
	var inode filesystem.SpecialInode
	inode.Mode = filesystem.FileMode(stat.Mode)
	inode.Uid = stat.Uid
	inode.Gid = stat.Gid
	inode.MtimeSeconds = int64(stat.Mtim.Sec)
	inode.MtimeNanoSeconds = int32(stat.Mtim.Nsec)
	inode.Rdev = stat.Rdev
	return &inode
}

func scanFileSystem(rootDirectoryName string,
	fsScanContext *fsrateio.ReaderContext, scanFilter *filter.Filter,
	checkScanDisableRequest func() bool, hasher Hasher, oldFS *FileSystem) (
	*FileSystem, error) {
	if checkScanDisableRequest != nil && checkScanDisableRequest() {
		return nil, errors.New("DisableScan")
	}
	var fileSystem FileSystem
	fileSystem.rootDirectoryName = rootDirectoryName
	fileSystem.fsScanContext = fsScanContext
	fileSystem.scanFilter = scanFilter
	fileSystem.checkScanDisableRequest = checkScanDisableRequest
	if hasher == nil {
		fileSystem.hasher = GetSimpleHasher(false)
	} else {
		fileSystem.hasher = hasher
	}
	var stat wsyscall.Stat_t
	if err := wsyscall.Lstat(rootDirectoryName, &stat); err != nil {
		return nil, err
	}
	fileSystem.InodeTable = make(filesystem.InodeTable)
	fileSystem.dev = stat.Dev
	fileSystem.inodeNumber = stat.Ino
	fileSystem.Mode = filesystem.FileMode(stat.Mode)
	fileSystem.Uid = stat.Uid
	fileSystem.Gid = stat.Gid
	fileSystem.DirectoryCount++
	var tmpInode filesystem.RegularInode
	if sha512.New().Size() != len(tmpInode.Hash) {
		return nil, errors.New("incompatible hash size")
	}
	var oldDirectory *filesystem.DirectoryInode
	if oldFS != nil && oldFS.InodeTable != nil {
		oldDirectory = &oldFS.DirectoryInode
	}
	err, _ := scanDirectory(&fileSystem.FileSystem.DirectoryInode, oldDirectory,
		&fileSystem, oldFS, "/")
	oldFS = nil
	if err != nil {
		return nil, err
	}
	fileSystem.ComputeTotalDataBytes()
	if err = fileSystem.RebuildInodePointers(); err != nil {
		panic(err)
	}
	return &fileSystem, nil
}

func scanDirectory(directory, oldDirectory *filesystem.DirectoryInode,
	fileSystem, oldFS *FileSystem, myPathName string) (error, bool) {
	file, err := os.Open(path.Join(fileSystem.rootDirectoryName, myPathName))
	if err != nil {
		return err, false
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return err, false
	}
	sort.Strings(names)
	entryList := make([]*filesystem.DirectoryEntry, 0, len(names))
	var copiedDirents int
	for _, name := range names {
		if directory == &fileSystem.DirectoryInode && name == ".subd" {
			continue
		}
		filename := path.Join(myPathName, name)
		if fileSystem.scanFilter != nil &&
			fileSystem.scanFilter.Match(filename) {
			continue
		}
		var stat wsyscall.Stat_t
		err := wsyscall.Lstat(path.Join(fileSystem.rootDirectoryName, filename),
			&stat)
		if err != nil {
			if err == syscall.ENOENT {
				continue
			}
			return err, false
		}
		if stat.Dev != fileSystem.dev {
			continue
		}
		if fileSystem.checkScanDisableRequest != nil &&
			fileSystem.checkScanDisableRequest() {
			return errors.New("DisableScan"), false
		}
		dirent := new(filesystem.DirectoryEntry)
		dirent.Name = name
		dirent.InodeNumber = stat.Ino
		var oldDirent *filesystem.DirectoryEntry
		if oldDirectory != nil {
			index := len(entryList)
			if len(oldDirectory.EntryList) > index &&
				oldDirectory.EntryList[index].Name == name {
				oldDirent = oldDirectory.EntryList[index]
			}
		}
		if stat.Mode&syscall.S_IFMT == syscall.S_IFDIR {
			err = addDirectory(dirent, oldDirent, fileSystem, oldFS, myPathName,
				&stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFREG {
			err = addRegularFile(dirent, fileSystem, oldFS, myPathName, &stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFLNK {
			err = addSymlink(dirent, fileSystem, oldFS, myPathName, &stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFSOCK {
			continue
		} else {
			err = addSpecialFile(dirent, fileSystem, oldFS, &stat)
		}
		if err != nil {
			if err == syscall.ENOENT {
				continue
			}
			return err, false
		}
		if oldDirent != nil && *dirent == *oldDirent {
			dirent = oldDirent
			copiedDirents++
		}
		entryList = append(entryList, dirent)
	}
	if oldDirectory != nil && len(entryList) == copiedDirents &&
		len(entryList) == len(oldDirectory.EntryList) {
		directory.EntryList = oldDirectory.EntryList
		return nil, true
	} else {
		directory.EntryList = entryList
		return nil, false
	}
}

func addDirectory(dirent, oldDirent *filesystem.DirectoryEntry,
	fileSystem, oldFS *FileSystem,
	directoryPathName string, stat *wsyscall.Stat_t) error {
	myPathName := path.Join(directoryPathName, dirent.Name)
	if stat.Ino == fileSystem.inodeNumber {
		return errors.New("recursive directory: " + myPathName)
	}
	if _, ok := fileSystem.InodeTable[stat.Ino]; ok {
		return errors.New("hardlinked directory: " + myPathName)
	}
	inode := new(filesystem.DirectoryInode)
	dirent.SetInode(inode)
	fileSystem.InodeTable[stat.Ino] = inode
	inode.Mode = filesystem.FileMode(stat.Mode)
	inode.Uid = stat.Uid
	inode.Gid = stat.Gid
	var oldInode *filesystem.DirectoryInode
	if oldDirent != nil {
		if oi, ok := oldDirent.Inode().(*filesystem.DirectoryInode); ok {
			oldInode = oi
		}
	}
	err, copied := scanDirectory(inode, oldInode, fileSystem, oldFS, myPathName)
	if err != nil {
		return err
	}
	if copied && filesystem.CompareDirectoriesMetadata(inode, oldInode, nil) {
		dirent.SetInode(oldInode)
		fileSystem.InodeTable[stat.Ino] = oldInode
	}
	fileSystem.DirectoryCount++
	return nil
}

func addRegularFile(dirent *filesystem.DirectoryEntry,
	fileSystem, oldFS *FileSystem,
	directoryPathName string, stat *wsyscall.Stat_t) error {
	if inode, ok := fileSystem.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			dirent.SetInode(inode)
			return nil
		}
		return errors.New("inode changed type: " + dirent.Name)
	}
	inode := makeRegularInode(stat)
	if inode.Size > 0 {
		err := scanRegularInode(inode, fileSystem,
			path.Join(directoryPathName, dirent.Name))
		if err != nil {
			return err
		}
	}
	if oldFS != nil && oldFS.InodeTable != nil {
		if oldInode, found := oldFS.InodeTable[stat.Ino]; found {
			if oldInode, ok := oldInode.(*filesystem.RegularInode); ok {
				if filesystem.CompareRegularInodes(inode, oldInode, nil) {
					inode = oldInode
				}
			}
		}
	}
	dirent.SetInode(inode)
	fileSystem.InodeTable[stat.Ino] = inode
	return nil
}

func addSymlink(dirent *filesystem.DirectoryEntry,
	fileSystem, oldFS *FileSystem,
	directoryPathName string, stat *wsyscall.Stat_t) error {
	if inode, ok := fileSystem.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.SymlinkInode); ok {
			dirent.SetInode(inode)
			return nil
		}
		return errors.New("inode changed type: " + dirent.Name)
	}
	inode := makeSymlinkInode(stat)
	err := scanSymlinkInode(inode, fileSystem,
		path.Join(directoryPathName, dirent.Name))
	if err != nil {
		return err
	}
	if oldFS != nil && oldFS.InodeTable != nil {
		if oldInode, found := oldFS.InodeTable[stat.Ino]; found {
			if oldInode, ok := oldInode.(*filesystem.SymlinkInode); ok {
				if filesystem.CompareSymlinkInodes(inode, oldInode, nil) {
					inode = oldInode
				}
			}
		}
	}
	dirent.SetInode(inode)
	fileSystem.InodeTable[stat.Ino] = inode
	return nil
}

func addSpecialFile(dirent *filesystem.DirectoryEntry,
	fileSystem, oldFS *FileSystem, stat *wsyscall.Stat_t) error {
	if inode, ok := fileSystem.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.SpecialInode); ok {
			dirent.SetInode(inode)
			return nil
		}
		return errors.New("inode changed type: " + dirent.Name)
	}
	inode := makeSpecialInode(stat)
	if oldFS != nil && oldFS.InodeTable != nil {
		if oldInode, found := oldFS.InodeTable[stat.Ino]; found {
			if oldInode, ok := oldInode.(*filesystem.SpecialInode); ok {
				if filesystem.CompareSpecialInodes(inode, oldInode, nil) {
					inode = oldInode
				}
			}
		}
	}
	dirent.SetInode(inode)
	fileSystem.InodeTable[stat.Ino] = inode
	return nil
}

func (h simpleHasher) hash(reader io.Reader, length uint64) (hash.Hash, error) {
	hasher := sha512.New()
	var hashVal hash.Hash
	nCopied, err := io.CopyN(hasher, reader, int64(length))
	if err != nil && err != io.EOF {
		return hashVal, err
	}
	if nCopied != int64(length) {
		if h {
			// File changed length. Don't interrupt the scanning: return the
			// zero hash and keep going. Hopefully next scan the file will be
			// stable.
			return hashVal, nil
		}
		return hashVal, fmt.Errorf("read: %d, expected: %d bytes",
			nCopied, length)
	}
	copy(hashVal[:], hasher.Sum(nil))
	return hashVal, nil
}

func (h cpuLimitedHasher) hash(reader io.Reader, length uint64) (
	hash.Hash, error) {
	h.limiter.Limit()
	return h.hasher.Hash(reader, length)
}

func scanRegularInode(inode *filesystem.RegularInode, fileSystem *FileSystem,
	myPathName string) error {
	pathName := path.Join(fileSystem.rootDirectoryName, myPathName)
	if oh, ok := fileSystem.hasher.(openingHasher); ok {
		if hashed, err := oh.OpenAndHash(inode, pathName); err != nil {
			return err
		} else if hashed {
			return nil
		}
	}
	f, err := os.Open(pathName)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := io.Reader(f)
	if fileSystem.fsScanContext != nil {
		reader = fileSystem.fsScanContext.NewReader(f)
	}
	inode.Hash, err = fileSystem.hasher.Hash(reader, inode.Size)
	if err != nil {
		return fmt.Errorf("scanRegularInode(%s): %s", myPathName, err)
	}
	return nil
}

func scanSymlinkInode(inode *filesystem.SymlinkInode, fileSystem *FileSystem,
	myPathName string) error {
	target, err := os.Readlink(path.Join(fileSystem.rootDirectoryName,
		myPathName))
	if err != nil {
		return err
	}
	inode.Symlink = target
	return nil
}
