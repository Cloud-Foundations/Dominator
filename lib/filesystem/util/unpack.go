package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

const (
	dirPerms  = syscall.S_IRWXU
	filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP
)

func createEmptyFile(filename string) error {
	if file, err := os.Create(filename); err != nil {
		return err
	} else {
		// Don't wait for finaliser to close, otherwise we can have too many
		// open files.
		file.Close()
		return nil
	}
}

func unpack(fs *filesystem.FileSystem, objectsGetter objectserver.ObjectsGetter,
	dirname string, logger log.Logger) error {
	for _, entry := range fs.EntryList {
		if entry.Name == ".inodes" {
			return errors.New("cannot unpack a file-system with /.inodes")
		}
	}
	os.Mkdir(dirname, dirPerms)
	inodesDir := path.Join(dirname, ".inodes")
	if err := os.Mkdir(inodesDir, dirPerms); err != nil {
		return err
	}
	defer os.RemoveAll(inodesDir)
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(inodesDir, &statfs); err != nil {
		return fmt.Errorf("unable to Statfs: %s %s\n", inodesDir, err)
	}
	if fs.TotalDataBytes > uint64(statfs.Bsize)*statfs.Bfree {
		return errors.New("image will not fit on file-system")
	}
	hashes, inums, lengths := getHashes(fs)
	err := writeObjects(objectsGetter, hashes, inums, lengths, inodesDir,
		logger)
	if err != nil {
		return err
	}
	startWriteTime := time.Now()
	if err := writeInodes(fs.InodeTable, inodesDir); err != nil {
		return err
	}
	if err = fs.DirectoryInode.Write(dirname); err != nil {
		return err
	}
	startBuildTime := time.Now()
	writeDuration := startBuildTime.Sub(startWriteTime)
	err = buildTree(&fs.DirectoryInode, dirname, "", inodesDir)
	if err != nil {
		return err
	}
	buildDuration := time.Since(startBuildTime)
	logger.Printf("Unpacked file-system: made inodes in %s, built tree in %s\n",
		format.Duration(writeDuration), format.Duration(buildDuration))
	return nil
}

func getHashes(fs *filesystem.FileSystem) ([]hash.Hash, []uint64, []uint64) {
	hashes := make([]hash.Hash, 0, fs.NumRegularInodes)
	inums := make([]uint64, 0, fs.NumRegularInodes)
	lengths := make([]uint64, 0, fs.NumRegularInodes)
	for inum, inode := range fs.InodeTable {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				hashes = append(hashes, inode.Hash)
				inums = append(inums, inum)
				lengths = append(lengths, inode.Size)
			}
		}
	}
	return hashes, inums, lengths
}

func writeObjects(objectsGetter objectserver.ObjectsGetter, hashes []hash.Hash,
	inums []uint64, lengths []uint64, inodesDir string,
	logger log.Logger) error {
	startTime := time.Now()
	objectsReader, err := objectsGetter.GetObjects(hashes)
	if err != nil {
		return fmt.Errorf("error getting object reader: %s\n", err)
	}
	defer objectsReader.Close()
	var totalLength uint64
	buffer := make([]byte, 32<<10)
	for index := range hashes {
		err = writeObject(objectsReader, inums[index], lengths[index],
			inodesDir, buffer)
		if err != nil {
			return err
		}
		totalLength += lengths[index]
	}
	duration := time.Since(startTime)
	speed := uint64(float64(totalLength) / duration.Seconds())
	logger.Printf("Copied %d objects (%s) in %s (%s/s)\n",
		len(hashes), format.FormatBytes(totalLength), format.Duration(duration),
		format.FormatBytes(speed))
	return nil
}

func writeObject(objectsReader objectserver.ObjectsReader, inodeNumber uint64,
	length uint64, inodesDir string, buffer []byte) error {
	rlength, reader, err := objectsReader.NextObject()
	if err != nil {
		return err
	}
	defer reader.Close()
	if rlength != length {
		return errors.New("mismatched lengths")
	}
	filename := path.Join(inodesDir, fmt.Sprintf("%d", inodeNumber))
	destFile, err := os.OpenFile(filename,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerms)
	if err != nil {
		return err
	}
	doClose := true
	defer func() {
		if doClose {
			destFile.Close()
		}
	}()
	iLength := int64(length)
	nCopied, err := io.CopyBuffer(destFile, io.LimitReader(reader, iLength),
		buffer)
	if err != nil {
		return fmt.Errorf("error copying: %s", err)
	}
	if nCopied != iLength {
		return fmt.Errorf("expected length: %d, got: %d for: %s\n",
			length, nCopied, filename)
	}
	doClose = false
	return destFile.Close()
}

func writeInode(inode filesystem.GenericInode, filename string) error {
	switch inode := inode.(type) {
	case *filesystem.RegularInode:
		if inode.Size < 1 {
			if err := createEmptyFile(filename); err != nil {
				return err
			}
		}
		if err := inode.WriteMetadata(filename); err != nil {
			return err
		}
	case *filesystem.ComputedRegularInode:
		if err := createEmptyFile(filename); err != nil {
			return err
		}
		tmpInode := &filesystem.RegularInode{
			Mode: inode.Mode,
			Uid:  inode.Uid,
			Gid:  inode.Gid,
		}
		if err := tmpInode.WriteMetadata(filename); err != nil {
			return err
		}
	case *filesystem.SymlinkInode:
		if err := inode.Write(filename); err != nil {
			return err
		}
	case *filesystem.SpecialInode:
		if err := inode.Write(filename); err != nil {
			return err
		}
	case *filesystem.DirectoryInode:
		if err := inode.Write(filename); err != nil {
			return err
		}
	default:
		return errors.New("unsupported inode type")
	}
	return nil
}

func writeInodes(inodeTable filesystem.InodeTable, inodesDir string) error {
	for inodeNumber, inode := range inodeTable {
		filename := path.Join(inodesDir, fmt.Sprintf("%d", inodeNumber))
		if err := writeInode(inode, filename); err != nil {
			return err
		}
	}
	return nil
}

func buildTree(directory *filesystem.DirectoryInode,
	rootDir, mySubPathName, inodesDir string) error {
	for _, dirent := range directory.EntryList {
		oldPath := path.Join(inodesDir, fmt.Sprintf("%d", dirent.InodeNumber))
		newSubPath := path.Join(mySubPathName, dirent.Name)
		newFullPath := path.Join(rootDir, newSubPath)
		if inode := dirent.Inode(); inode == nil {
			panic("no inode pointer for: " + newSubPath)
		} else if dinode, ok := inode.(*filesystem.DirectoryInode); ok {
			if err := renameDir(oldPath, newFullPath, dinode); err != nil {
				return err
			}
			err := buildTree(dinode, rootDir, newSubPath, inodesDir)
			if err != nil {
				return err
			}
		} else {
			if err := link(oldPath, newFullPath, inode); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	return nil
}

func link(oldname, newname string, inode filesystem.GenericInode) error {
	if err := os.Link(oldname, newname); err == nil {
		return nil
	}
	if finode, ok := inode.(*filesystem.RegularInode); ok {
		if finode.Size > 0 {
			reader, err := os.Open(oldname)
			if err != nil {
				return err
			}
			defer reader.Close()
			writer, err := os.Create(newname)
			if err != nil {
				return err
			}
			defer writer.Close()
			if _, err := io.Copy(writer, reader); err != nil {
				return err
			}
		}
	}
	if err := writeInode(inode, newname); err != nil {
		return err
	}
	return nil
}

func renameDir(oldpath, newpath string,
	inode *filesystem.DirectoryInode) error {
	if err := os.Rename(oldpath, newpath); err == nil {
		return nil
	}
	if oldFi, err := os.Lstat(oldpath); err != nil {
		return err
	} else {
		if !oldFi.IsDir() {
			return fmt.Errorf("%s is not a directory", oldpath)
		}
	}
	if newFi, err := os.Lstat(newpath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := inode.Write(newpath); err != nil {
			return err
		}
	} else {
		if !newFi.IsDir() {
			return fmt.Errorf("%s is not a directory", newpath)
		}
		if err := inode.WriteMetadata(newpath); err != nil {
			return err
		}
	}
	if err := os.Remove(oldpath); err != nil {
		return err
	}
	return nil
}
