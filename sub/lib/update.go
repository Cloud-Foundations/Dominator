package lib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *uType) update(request sub.UpdateRequest) error {
	if request.Triggers == nil {
		request.Triggers = triggers.New()
	}
	if t.SkipFilter == nil {
		t.SkipFilter = new(filter.Filter)
	}
	t.copyFilesToCache(request.FilesToCopyToCache)
	t.makeObjectCopies(request.MultiplyUsedObjects)
	if t.RunTriggers != nil &&
		t.OldTriggers != nil && len(t.OldTriggers.Triggers) > 0 {
		t.makeDirectories(request.DirectoriesToMake,
			t.OldTriggers, false)
		t.makeInodes(request.InodesToMake, request.MultiplyUsedObjects,
			t.OldTriggers, false)
		t.makeHardlinks(request.HardlinksToMake, t.OldTriggers, false)
		t.doDeletes(request.PathsToDelete, t.OldTriggers, false)
		t.changeInodes(request.InodesToChange, t.OldTriggers, false)
		matchedOldTriggers := t.OldTriggers.GetMatchedTriggers()
		err := t.checkDisruption(matchedOldTriggers, request.ForceDisruption)
		if err != nil {
			return err
		}
		if t.RunTriggers(matchedOldTriggers, "stop", t.Logger) {
			t.hadTriggerFailures = true
		}
	}
	fsChangeStartTime := time.Now()
	t.makeDirectories(request.DirectoriesToMake, request.Triggers, true)
	t.makeInodes(request.InodesToMake, request.MultiplyUsedObjects,
		request.Triggers, true)
	t.makeHardlinks(request.HardlinksToMake, request.Triggers, true)
	t.doDeletes(request.PathsToDelete, request.Triggers, true)
	t.changeInodes(request.InodesToChange, request.Triggers, true)
	if err := t.writePatchedImageName(request.ImageName); err != nil {
		t.Logger.Println(err)
	}
	t.fsChangeDuration = time.Since(fsChangeStartTime)
	matchedNewTriggers := request.Triggers.GetMatchedTriggers()
	if t.RunTriggers != nil &&
		t.RunTriggers(matchedNewTriggers, "start", t.Logger) {
		t.hadTriggerFailures = true
	}
	return t.lastError
}

func (t *uType) checkDisruption(matchedTriggers []*triggers.Trigger,
	force bool) error {
	if t.DisruptionRequest == nil && t.DisruptionCancel == nil {
		return nil
	}
	if !isHighImpact(matchedTriggers) {
		if t.DisruptionCancel != nil {
			t.DisruptionCancel()
		}
		return nil
	}
	if force {
		return nil
	}
	if t.DisruptionRequest == nil {
		return nil
	}
	switch t.DisruptionRequest() {
	case sub.DisruptionStateAnytime:
		return nil
	case sub.DisruptionStatePermitted:
		return nil
	case sub.DisruptionStateRequested:
		return errors.New(sub.ErrorDisruptionPending)
	case sub.DisruptionStateDenied:
		return errors.New(sub.ErrorDisruptionDenied)
	default:
		return nil
	}
}

func isHighImpact(matchedTriggers []*triggers.Trigger) bool {
	if len(matchedTriggers) < 1 {
		return false
	}
	for _, trigger := range matchedTriggers {
		if trigger.HighImpact {
			return true
		}
	}
	return false
}

func (t *uType) copyFilesToCache(filesToCopyToCache []sub.FileToCopyToCache) {
	for _, fileToCopy := range filesToCopyToCache {
		sourcePathname := filepath.Join(t.RootDirectoryName, fileToCopy.Name)
		destPathname := filepath.Join(t.ObjectsDir,
			objectcache.HashToFilename(fileToCopy.Hash))
		prefix := "Copied"
		if fileToCopy.DoHardlink {
			prefix = "Hardlinked"
		}
		if err := copyFile(destPathname, sourcePathname,
			fileToCopy.DoHardlink); err != nil {
			t.lastError = err
			t.Logger.Println(err)
		} else {
			t.Logger.Printf("%s: %s to cache\n", prefix, sourcePathname)
		}
	}
}

func copyFile(destPathname, sourcePathname string, doHardlink bool) error {
	dirname := filepath.Dir(destPathname)
	if err := os.MkdirAll(dirname, syscall.S_IRWXU); err != nil {
		return err
	}
	if doHardlink {
		return fsutil.ForceLink(sourcePathname, destPathname)
	}
	sourceFile, err := os.Open(sourcePathname)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	destFile, err := os.Create(destPathname)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (t *uType) makeObjectCopies(multiplyUsedObjects map[hash.Hash]uint64) {
	for hash, numCopies := range multiplyUsedObjects {
		if numCopies < 2 {
			continue
		}
		objectPathname := filepath.Join(t.ObjectsDir,
			objectcache.HashToFilename(hash))
		for numCopies--; numCopies > 0; numCopies-- {
			ext := fmt.Sprintf("~%d~", numCopies)
			if err := copyFile(objectPathname+ext, objectPathname,
				false); err != nil {
				t.lastError = err
				t.Logger.Println(err)
			} else {
				t.Logger.Printf("Copied object: %x%s\n", hash, ext)
			}
		}
	}
}

func (t *uType) makeInodes(inodesToMake []sub.Inode,
	multiplyUsedObjects map[hash.Hash]uint64, triggers *triggers.Triggers,
	takeAction bool) {
	for _, inode := range inodesToMake {
		triggers.Match(inode.Name)
		if takeAction {
			fullPathname := filepath.Join(t.RootDirectoryName, inode.Name)
			var err error
			switch inode := inode.GenericInode.(type) {
			case *filesystem.RegularInode:
				err = makeRegularInode(fullPathname, inode, multiplyUsedObjects,
					t.ObjectsDir, t.Logger)
			case *filesystem.SymlinkInode:
				err = makeSymlinkInode(fullPathname, inode, t.Logger)
			case *filesystem.SpecialInode:
				err = makeSpecialInode(fullPathname, inode, t.Logger)
			}
			if err != nil {
				t.lastError = err
			}
		}
	}
}

func makeRegularInode(fullPathname string,
	inode *filesystem.RegularInode, multiplyUsedObjects map[hash.Hash]uint64,
	objectsDir string, logger log.Logger) error {
	var objectPathname string
	if inode.Size > 0 {
		objectPathname = filepath.Join(objectsDir,
			objectcache.HashToFilename(inode.Hash))
		numCopies := multiplyUsedObjects[inode.Hash]
		if numCopies > 1 {
			numCopies--
			objectPathname += fmt.Sprintf("~%d~", numCopies)
			if numCopies < 2 {
				delete(multiplyUsedObjects, inode.Hash)
			} else {
				multiplyUsedObjects[inode.Hash] = numCopies
			}
		}
	} else {
		objectPathname = fmt.Sprintf("%s.empty.%d", fullPathname, os.Getpid())
		if file, err := os.OpenFile(objectPathname,
			os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600); err != nil {
			return err
		} else {
			file.Close()
		}
	}
	if err := fsutil.ForceRename(objectPathname, fullPathname); err != nil {
		logger.Println(err)
		return err
	}
	if err := inode.WriteMetadata(fullPathname); err != nil {
		logger.Println(err)
		return err
	} else {
		if inode.Size > 0 {
			logger.Printf("Made inode: %s from: %x\n",
				fullPathname, inode.Hash)
		} else {
			logger.Printf("Made empty inode: %s\n", fullPathname)
		}
	}
	return nil
}

func makeSymlinkInode(fullPathname string,
	inode *filesystem.SymlinkInode, logger log.Logger) error {
	if err := inode.Write(fullPathname); err != nil {
		logger.Println(err)
		return err
	}
	logger.Printf("Made symlink inode: %s -> %s\n", fullPathname, inode.Symlink)
	return nil
}

func makeSpecialInode(fullPathname string, inode *filesystem.SpecialInode,
	logger log.Logger) error {
	if err := inode.Write(fullPathname); err != nil {
		logger.Println(err)
		return err
	}
	logger.Printf("Made special inode: %s\n", fullPathname)
	return nil
}

func (t *uType) makeHardlinks(hardlinksToMake []sub.Hardlink,
	triggers *triggers.Triggers, takeAction bool) {
	tmpName := filepath.Join(t.ObjectsDir, "temporaryHardlink")
	for _, hardlink := range hardlinksToMake {
		triggers.Match(hardlink.NewLink)
		if takeAction {
			targetPathname := filepath.Join(t.RootDirectoryName,
				hardlink.Target)
			linkPathname := filepath.Join(t.RootDirectoryName, hardlink.NewLink)
			// A Link directly to linkPathname will fail if it exists, so do a
			// Link+Rename using a temporary filename.
			if err := fsutil.ForceLink(targetPathname, tmpName); err != nil {
				t.lastError = err
				t.Logger.Println(err)
				continue
			}
			if err := fsutil.ForceRename(tmpName, linkPathname); err != nil {
				t.Logger.Println(err)
				if err := fsutil.ForceRemove(tmpName); err != nil {
					t.lastError = err
					t.Logger.Println(err)
				}
			} else {
				t.Logger.Printf("Linked: %s => %s\n",
					linkPathname, targetPathname)
			}
		}
	}
}

func (t *uType) doDeletes(pathsToDelete []string, triggers *triggers.Triggers,
	takeAction bool) {
	for _, pathname := range pathsToDelete {
		triggers.Match(pathname)
		if takeAction {
			fullPathname := filepath.Join(t.RootDirectoryName, pathname)
			if err := fsutil.ForceRemoveAll(fullPathname); err != nil {
				t.lastError = err
				t.Logger.Println(err)
			} else {
				t.Logger.Printf("Deleted: %s\n", fullPathname)
			}
		}
	}
}

func (t *uType) makeDirectories(directoriesToMake []sub.Inode,
	triggers *triggers.Triggers, takeAction bool) {
	for _, newdir := range directoriesToMake {
		if t.skipPath(newdir.Name) {
			continue
		}
		triggers.Match(newdir.Name)
		if takeAction {
			fullPathname := filepath.Join(t.RootDirectoryName, newdir.Name)
			inode, ok := newdir.GenericInode.(*filesystem.DirectoryInode)
			if !ok {
				t.Logger.Println("%s is not a directory!\n", newdir.Name)
				continue
			}
			if err := inode.Write(fullPathname); err != nil {
				t.lastError = err
				t.Logger.Println(err)
			} else {
				t.Logger.Printf("Made directory: %s (mode=%s)\n",
					fullPathname, inode.Mode)
			}
		}
	}
}

func (t *uType) changeInodes(inodesToChange []sub.Inode,
	triggers *triggers.Triggers, takeAction bool) {
	for _, inode := range inodesToChange {
		fullPathname := filepath.Join(t.RootDirectoryName, inode.Name)
		if checkNonMtimeChange(fullPathname, inode.GenericInode) {
			triggers.Match(inode.Name)
		}
		if takeAction {
			if err := filesystem.ForceWriteMetadata(inode,
				fullPathname); err != nil {
				t.lastError = err
				t.Logger.Println(err)
				continue
			}
			t.Logger.Printf("Changed inode: %s\n", fullPathname)
		}
	}
}

func checkNonMtimeChange(filename string, inode filesystem.GenericInode) bool {
	switch inode := inode.(type) {
	case *filesystem.RegularInode:
		var stat wsyscall.Stat_t
		if err := wsyscall.Lstat(filename, &stat); err != nil {
			return true
		}
		if stat.Mode&syscall.S_IFMT == syscall.S_IFREG {
			oldInode := scanner.MakeRegularInode(&stat)
			oldInode.Hash = inode.Hash
			oldInode.MtimeNanoSeconds = inode.MtimeNanoSeconds
			oldInode.MtimeSeconds = inode.MtimeSeconds
			if *oldInode == *inode {
				return false
			}
		}
	case *filesystem.SpecialInode:
		var stat wsyscall.Stat_t
		if err := wsyscall.Lstat(filename, &stat); err != nil {
			return true
		}
		if stat.Mode&syscall.S_IFMT == syscall.S_IFBLK ||
			stat.Mode&syscall.S_IFMT == syscall.S_IFCHR {
			oldInode := scanner.MakeSpecialInode(&stat)
			oldInode.MtimeNanoSeconds = inode.MtimeNanoSeconds
			oldInode.MtimeSeconds = inode.MtimeSeconds
			if *oldInode == *inode {
				return false
			}
		}
	}
	return true
}

func (t *uType) skipPath(pathname string) bool {
	if t.SkipFilter.Match(pathname) {
		return true
	}
	if pathname == "/.subd" {
		return true
	}
	if strings.HasPrefix(pathname, "/.subd/") {
		return true
	}
	return false
}

func (t *uType) writePatchedImageName(imageName string) error {
	pathname := filepath.Join(t.RootDirectoryName,
		constants.PatchedImageNameFile)
	if imageName == "" {
		if err := os.Remove(pathname); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(pathname), fsutil.DirPerms); err != nil {
		return err
	}
	buffer := &bytes.Buffer{}
	fmt.Fprintln(buffer, imageName)
	return fsutil.CopyToFile(pathname, fsutil.PublicFilePerms, buffer, 0)
}
