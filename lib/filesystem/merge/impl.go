package merge

import (
	"fmt"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/build"
)

var (
	defaultOptions = Options{
		PrefixPath: "/",
	}
)

func newMerger(optionsPtr *Options) (*Merger, error) {
	options, err := validateOptions(optionsPtr)
	if err != nil {
		return nil, err
	}
	m := &Merger{
		builder:    build.New(),
		options:    options,
		inodeTable: make(map[string]filesystem.GenericInode),
	}
	m.inodeTable["/"] = &m.builder.FileSystem.DirectoryInode
	return m, nil
}

func validateOptions(optionsPtr *Options) (Options, error) {
	if optionsPtr == nil {
		return defaultOptions, nil
	}
	if optionsPtr.PrefixPath[0] != '/' {
		return Options{},
			fmt.Errorf("prefix: \"%s\" is not rooted", optionsPtr.PrefixPath)
	}
	return *optionsPtr, nil
}

// addPrefixDirectory will add a missing directory and return the parent.
func (m *Merger) addPrefixDirectory(dirname string) error {
	parentName, leafName := path.Split(dirname)
	parentName = path.Clean(parentName)
	inode, ok := m.inodeTable[dirname]
	if !ok {
		if err := m.addPrefixDirectory(parentName); err != nil {
			return err
		}
		parent := m.inodeTable[parentName].(*filesystem.DirectoryInode)
		inode := &filesystem.DirectoryInode{
			Mode: m.builder.FileSystem.Mode,
		}
		if err := m.builder.AddInode(inode); err != nil {
			return err
		}
		m.inodeTable[dirname] = inode
		return m.builder.AddDirectoryEntry(parent, leafName, inode)
	}
	if _, ok := inode.(*filesystem.DirectoryInode); !ok {
		return fmt.Errorf("%s is not a directory", dirname)
	}
	return nil
}

func (m *Merger) compareInodes(
	lowerInode, upperInode filesystem.GenericInode) bool {
	sameType, sameMetadata, sameData := filesystem.CompareInodesIgnoreMtimes(
		lowerInode, upperInode, nil)
	if !sameType || !sameMetadata {
		return false
	}
	if _, isDir := upperInode.(*filesystem.DirectoryInode); isDir {
		return true // Don't care about "data" (directory entries).
	}
	return sameData
}

func (m *Merger) getFileSystem() *filesystem.FileSystem {
	m.builder.Sort()
	return m.builder.FileSystem
}

func (m *Merger) merge(fs *filesystem.FileSystem, optionsPtr *Options) error {
	if fs == nil {
		return fmt.Errorf("nil FileSystem")
	}
	if optionsPtr == nil {
		return m.mergeDirectory(&fs.DirectoryInode, m.options.PrefixPath)
	}
	options, err := validateOptions(optionsPtr)
	if err != nil {
		return err
	}
	oldOptions := m.options
	defer func() {
		m.options = oldOptions
	}()
	m.options = options
	if err := m.addPrefixDirectory(m.options.PrefixPath); err != nil {
		return err
	}
	return m.mergeDirectory(&fs.DirectoryInode, m.options.PrefixPath)
}

func (m *Merger) mergeDirectory(upperDirectory *filesystem.DirectoryInode,
	dirname string) error {
	lowerDirectoryInode, ok := m.inodeTable[dirname]
	if !ok {
		return fmt.Errorf("missing inode in lower layer: %s", dirname)
	}
	lowerDirectory, ok := lowerDirectoryInode.(*filesystem.DirectoryInode)
	if !ok {
		return fmt.Errorf("inode in lower layer is not a directory: %s",
			dirname)
	}
	for _, entry := range upperDirectory.EntryList {
		err := m.mergeEntry(lowerDirectory, upperDirectory, dirname, entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Merger) mergeEntry(lowerDirectory *filesystem.DirectoryInode,
	upperDirectory *filesystem.DirectoryInode, dirname string,
	upperEntry *filesystem.DirectoryEntry) error {
	pathname := path.Join(dirname, upperEntry.Name)
	lowerInode, ok := m.inodeTable[pathname]
	upperInode := upperEntry.Inode()
	inodeToAdd := upperInode
	if inode, ok := inodeToAdd.(*filesystem.DirectoryInode); ok {
		// Replace inode with empty directory to ensure that entries are added
		// by walking only and not direct inheriting.
		inodeToAdd = &filesystem.DirectoryInode{
			Mode: inode.Mode,
			Uid:  inode.Uid,
			Gid:  inode.Gid,
		}
	}
	if !ok {
		// Simple case: lower entry doesn't exist. Just add it.
		if err := m.builder.AddInode(inodeToAdd); err != nil {
			return fmt.Errorf("%s: %s", pathname, err)
		}
		err := m.builder.AddDirectoryEntry(lowerDirectory, upperEntry.Name,
			inodeToAdd)
		if err != nil {
			return fmt.Errorf("%s: %s", pathname, err)
		}
		m.inodeTable[pathname] = inodeToAdd
	} else if !m.compareInodes(lowerInode, upperInode) {
		return fmt.Errorf(
			"inode in upper layer is different than lower layer: %s",
			pathname)
	}
	if dir, ok := upperInode.(*filesystem.DirectoryInode); ok {
		return m.mergeDirectory(dir, pathname)
	}
	return nil
}
