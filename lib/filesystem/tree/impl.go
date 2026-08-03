package tree

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fstree"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type decoderData struct {
	directoryTable  map[string]*filesystem.DirectoryInode
	fileSystem      filesystem.FileSystem
	nextInodeNumber uint64
	startTime       time.Time
}

func get(getter fstree.Getter, treeUrl string, logger log.DebugLogger) (
	*filesystem.FileSystem, []hash.Hash, []uint64, error) {
	dd := decoderData{
		directoryTable: make(map[string]*filesystem.DirectoryInode),
		startTime:      time.Now(),
	}
	fileSystem := &dd.fileSystem
	fileSystem.InodeTable = make(filesystem.InodeTable)
	// Create a default top-level directory.
	dd.addInode(&fileSystem.DirectoryInode)
	fileSystem.DirectoryInode.Mode = wsyscall.S_IFDIR | wsyscall.S_IRWXU |
		wsyscall.S_IRGRP | wsyscall.S_IXGRP | wsyscall.S_IROTH |
		wsyscall.S_IXOTH
	dd.directoryTable["/"] = &fileSystem.DirectoryInode
	// Start walking trees.
	hashes := make(map[hash.Hash]uint64)
	var numBytes, numDirectories, numSymlinks uint64
	var mutex sync.Mutex
	fn := func(getter fstree.Getter, dirname string,
		entry *fstree.TreeEntry) error {
		switch entry.Type {
		case fstree.TypeBlob:
			mutex.Lock()
			if entry.Size != 0 {
				hashes[entry.Hash] = entry.Size
			}
			fileSystem.NumRegularInodes++
			fileSystem.TotalDataBytes += entry.Size
			if err := dd.addFile(dirname, entry); err != nil {
				return err
			}
			mutex.Unlock()
		case fstree.TypeTree:
			mutex.Lock()
			numDirectories++
			numBytes += entry.Size
			if err := dd.addDirectory(dirname, entry); err != nil {
				return err
			}
			mutex.Unlock()
		case fstree.TypeSymlink:
			data, err := getter.GetBlobData(entry.Hash)
			if err != nil {
				return err
			}
			mutex.Lock()
			numBytes += entry.Size
			numSymlinks++
			if err := dd.addSymlink(dirname, entry, data); err != nil {
				return err
			}
			mutex.Unlock()
		}
		return nil
	}
	walkParams := fstree.WalkParams{
		Function: fn,
		Getter:   getter,
		Logger:   logger,
		TreeUrl:  treeUrl,
	}
	if err := fstree.WalkTree(walkParams); err != nil {
		return nil, nil, nil, err
	}
	duration := time.Since(dd.startTime)
	speed := float64(numBytes) / duration.Seconds()
	logger.Printf("Fetched tree: numBytes: %s in: %ds (%s) (%s/s), numFiles: %d, numDirectories: %d, numSymlinks: %d\n",
		format.FormatBytes(numBytes),
		duration/time.Second,
		format.Duration(duration),
		format.FormatBytes(uint64(speed)),
		fileSystem.NumRegularInodes, numDirectories, numSymlinks,
	)
	logger.Printf("Num unique objects: %d\n", len(hashes))
	hashList := make([]hash.Hash, 0, len(hashes))
	objectSizes := make([]uint64, 0, len(hashes))
	for hashVal, size := range hashes {
		hashList = append(hashList, hashVal)
		objectSizes = append(objectSizes, size)
	}
	return fileSystem, hashList, objectSizes, nil
}

func (dd *decoderData) addDirectory(dirname string,
	entry *fstree.TreeEntry) error {
	var newInode filesystem.DirectoryInode
	newInode.Mode = filesystem.FileMode(
		(entry.Permissions & ^uint32(wsyscall.S_IFMT)) |
			uint32(wsyscall.S_IFDIR))
	newInode.Uid = entry.UserId
	newInode.Gid = entry.GroupId
	dd.addEntry(dirname, entry.Filename, &newInode)
	dd.directoryTable[filepath.Join(dirname, entry.Filename)] = &newInode
	return nil
}

func (dd *decoderData) addEntry(dirname, name string,
	inode filesystem.GenericInode) error {
	parent, ok := dd.directoryTable[dirname]
	if !ok {
		return fmt.Errorf("no parent directory found for: %s", dirname)
	}
	var newEntry filesystem.DirectoryEntry
	newEntry.Name = name
	newEntry.InodeNumber = dd.nextInodeNumber
	newEntry.SetInode(inode)
	parent.EntryList = append(parent.EntryList, &newEntry)
	dd.addInode(inode)
	return nil
}

func (dd *decoderData) addFile(dirname string, entry *fstree.TreeEntry) error {
	var newInode filesystem.RegularInode
	newInode.Mode = filesystem.FileMode(
		(entry.Permissions & ^uint32(wsyscall.S_IFMT)) |
			uint32(wsyscall.S_IFREG))
	newInode.Uid = entry.UserId
	newInode.Gid = entry.GroupId
	newInode.MtimeNanoSeconds = int32(dd.startTime.Nanosecond())
	newInode.MtimeSeconds = dd.startTime.Unix()
	newInode.Size = entry.Size
	newInode.Hash = entry.Hash
	dd.addEntry(dirname, entry.Filename, &newInode)
	return nil
}

func (dd *decoderData) addInode(inode filesystem.GenericInode) {
	dd.fileSystem.InodeTable[dd.nextInodeNumber] = inode
	dd.nextInodeNumber++
}

func (dd *decoderData) addSymlink(dirname string, entry *fstree.TreeEntry,
	data []byte) error {
	var newInode filesystem.SymlinkInode
	newInode.Uid = entry.UserId
	newInode.Gid = entry.GroupId
	newInode.Symlink = string(data)
	dd.addEntry(dirname, entry.Filename, &newInode)
	return nil
}
