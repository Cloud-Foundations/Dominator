package scanner

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"sync"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type nilLocker struct{}

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

func scanFileSystem(params Params) (*FileSystem, error) {
	if params.CheckScanDisableRequest != nil &&
		params.CheckScanDisableRequest() {
		return nil, errors.New("DisableScan")
	}
	if params.Hasher == nil {
		params.Hasher = GetSimpleHasher(false)
	}
	fileSystem := FileSystem{
		hashWaiters: make(map[uint64]<-chan struct{}),
	}
	if params.Runner == nil {
		params.Runner = concurrent.NewAutoScaler(1)
		fileSystem.fsLock = &nilLocker{}
	} else {
		fileSystem.fsLock = &sync.Mutex{}
	}
	fileSystem.params = params
	var stat wsyscall.Stat_t
	if err := wsyscall.Lstat(params.RootDirectoryName, &stat); err != nil {
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
	if params.OldFS != nil && params.OldFS.InodeTable != nil {
		oldDirectory = &params.OldFS.DirectoryInode
	}
	err, _ := fileSystem.scanDirectory(&fileSystem.FileSystem.DirectoryInode,
		oldDirectory, "/")
	params.OldFS = nil // Indicate early garbage collection.
	if err != nil {
		return nil, err
	}
	if err := params.Runner.Reap(); err != nil {
		return nil, err
	}
	fileSystem.ComputeTotalDataBytes()
	if err = fileSystem.RebuildInodePointers(); err != nil {
		return nil, err
	}
	return &fileSystem, nil
}

func (fs *FileSystem) scanDirectory(directory *filesystem.DirectoryInode,
	oldDirectory *filesystem.DirectoryInode, myPathName string) (error, bool) {
	file, err := os.Open(path.Join(fs.params.RootDirectoryName,
		myPathName))
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
		if directory == &fs.DirectoryInode && name == ".subd" {
			continue
		}
		filename := path.Join(myPathName, name)
		if fs.params.ScanFilter != nil &&
			fs.params.ScanFilter.Match(filename) {
			continue
		}
		var stat wsyscall.Stat_t
		err := wsyscall.Lstat(path.Join(fs.params.RootDirectoryName,
			filename), &stat)
		if err != nil {
			if err == syscall.ENOENT {
				continue
			}
			return err, false
		}
		if stat.Dev != fs.dev {
			continue
		}
		if fs.params.CheckScanDisableRequest != nil &&
			fs.params.CheckScanDisableRequest() {
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
			err = fs.addDirectory(dirent, oldDirent, myPathName, &stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFREG {
			err = fs.addRegularFile(dirent, myPathName, &stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFLNK {
			err = fs.addSymlink(dirent, myPathName, &stat)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFSOCK {
			continue
		} else {
			err = fs.addSpecialFile(dirent, &stat)
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

func (fs *FileSystem) addDirectory(dirent *filesystem.DirectoryEntry,
	oldDirent *filesystem.DirectoryEntry, directoryPathName string,
	stat *wsyscall.Stat_t) error {
	myPathName := path.Join(directoryPathName, dirent.Name)
	if stat.Ino == fs.inodeNumber {
		return errors.New("recursive directory: " + myPathName)
	}
	inode := new(filesystem.DirectoryInode)
	fs.fsLock.Lock()
	if _, ok := fs.InodeTable[stat.Ino]; ok {
		fs.fsLock.Unlock()
		return errors.New("hardlinked directory: " + myPathName)
	}
	fs.InodeTable[stat.Ino] = inode
	fs.fsLock.Unlock()
	dirent.SetInode(inode)
	inode.Mode = filesystem.FileMode(stat.Mode)
	inode.Uid = stat.Uid
	inode.Gid = stat.Gid
	var oldInode *filesystem.DirectoryInode
	if oldDirent != nil {
		if oi, ok := oldDirent.Inode().(*filesystem.DirectoryInode); ok {
			oldInode = oi
		}
	}
	err, copied := fs.scanDirectory(inode, oldInode, myPathName)
	if err != nil {
		return err
	}
	if copied && filesystem.CompareDirectoriesMetadata(inode, oldInode, nil) {
		dirent.SetInode(oldInode)
		fs.fsLock.Lock()
		fs.InodeTable[stat.Ino] = oldInode
		fs.fsLock.Unlock()
	}
	fs.DirectoryCount++
	return nil
}

func (fs *FileSystem) addRegularFile(dirent *filesystem.DirectoryEntry,
	directoryPathName string, stat *wsyscall.Stat_t) error {
	fs.fsLock.Lock()
	if ch := fs.hashWaiters[stat.Ino]; ch != nil {
		fs.fsLock.Unlock()
		<-ch
		fs.fsLock.Lock()
	}
	if inode, ok := fs.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			dirent.SetInode(inode)
			fs.fsLock.Unlock()
			return nil
		}
		fs.fsLock.Unlock()
		return errors.New("inode changed type: " + dirent.Name)
	}
	channel := make(chan struct{})
	fs.hashWaiters[stat.Ino] = channel
	fs.fsLock.Unlock()
	pathName := path.Join(fs.params.RootDirectoryName, directoryPathName,
		dirent.Name)
	file, err := os.Open(pathName)
	if err != nil {
		close(channel)
		return err
	}
	inode := makeRegularInode(stat)
	err = fs.params.Runner.GoRun(func() (uint64, error) {
		defer close(channel)
		defer file.Close()
		if inode.Size > 0 {
			err := fs.scanRegularInode(inode, file, stat)
			if err != nil {
				return 0, err
			}
		}
		if fs.params.OldFS != nil && fs.params.OldFS.InodeTable != nil {
			if oldInode, found := fs.InodeTable[stat.Ino]; found {
				if oldInode, ok := oldInode.(*filesystem.RegularInode); ok {
					if filesystem.CompareRegularInodes(inode, oldInode, nil) {
						inode = oldInode
					}
				}
			}
		}
		dirent.SetInode(inode)
		fs.fsLock.Lock()
		fs.InodeTable[stat.Ino] = inode
		fs.fsLock.Unlock()
		return inode.Size, nil
	})
	return err
}

func (fs *FileSystem) addSymlink(dirent *filesystem.DirectoryEntry,
	directoryPathName string, stat *wsyscall.Stat_t) error {
	fs.fsLock.Lock()
	if inode, ok := fs.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.SymlinkInode); ok {
			dirent.SetInode(inode)
			fs.fsLock.Unlock()
			return nil
		}
		fs.fsLock.Unlock()
		return errors.New("inode changed type: " + dirent.Name)
	}
	fs.fsLock.Unlock()
	inode := makeSymlinkInode(stat)
	err := fs.scanSymlinkInode(inode, path.Join(directoryPathName, dirent.Name))
	if err != nil {
		return err
	}
	if fs.params.OldFS != nil && fs.params.OldFS.InodeTable != nil {
		if oldInode, found := fs.params.OldFS.InodeTable[stat.Ino]; found {
			if oldInode, ok := oldInode.(*filesystem.SymlinkInode); ok {
				if filesystem.CompareSymlinkInodes(inode, oldInode, nil) {
					inode = oldInode
				}
			}
		}
	}
	dirent.SetInode(inode)
	fs.fsLock.Lock()
	fs.InodeTable[stat.Ino] = inode
	fs.fsLock.Unlock()
	return nil
}

func (fs *FileSystem) addSpecialFile(dirent *filesystem.DirectoryEntry,
	stat *wsyscall.Stat_t) error {
	fs.fsLock.Lock()
	if inode, ok := fs.InodeTable[stat.Ino]; ok {
		if inode, ok := inode.(*filesystem.SpecialInode); ok {
			dirent.SetInode(inode)
			fs.fsLock.Unlock()
			return nil
		}
		fs.fsLock.Unlock()
		return errors.New("inode changed type: " + dirent.Name)
	}
	fs.fsLock.Unlock()
	inode := makeSpecialInode(stat)
	if fs.params.OldFS != nil && fs.params.OldFS.InodeTable != nil {
		if oldInode, found := fs.params.OldFS.InodeTable[stat.Ino]; found {
			if oldInode, ok := oldInode.(*filesystem.SpecialInode); ok {
				if filesystem.CompareSpecialInodes(inode, oldInode, nil) {
					inode = oldInode
				}
			}
		}
	}
	dirent.SetInode(inode)
	fs.fsLock.Lock()
	fs.InodeTable[stat.Ino] = inode
	fs.fsLock.Unlock()
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

func (fs *FileSystem) scanRegularInode(inode *filesystem.RegularInode,
	file *os.File, stat *wsyscall.Stat_t) error {
	if rh, ok := fs.params.Hasher.(readingHasher); ok {
		if hashed, err := rh.ReadAndHash(inode, file, stat); err != nil {
			return err
		} else if hashed {
			return nil
		}
	}
	reader := io.Reader(file)
	if fs.params.FsScanContext != nil {
		reader = fs.params.FsScanContext.NewReader(file)
	}
	var err error
	inode.Hash, err = fs.params.Hasher.Hash(reader, inode.Size)
	if err != nil {
		return fmt.Errorf("scanRegularInode(%s): %s", file.Name(), err)
	}
	return nil
}

func (fs *FileSystem) scanSymlinkInode(inode *filesystem.SymlinkInode,
	myPathName string) error {
	target, err := os.Readlink(path.Join(fs.params.RootDirectoryName,
		myPathName))
	if err != nil {
		return err
	}
	inode.Symlink = target
	return nil
}

func (l nilLocker) Lock() {}

func (l nilLocker) Unlock() {}
