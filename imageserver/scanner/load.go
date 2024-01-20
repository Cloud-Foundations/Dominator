package scanner

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/logutil"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func loadImageDataBase(config Config, params Params) (*ImageDataBase, error) {
	fi, err := os.Stat(config.BaseDirectory)
	if err != nil {
		return nil, fmt.Errorf("cannot stat: %s: %s\n",
			config.BaseDirectory, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory\n", config.BaseDirectory)
	}
	imdb := &ImageDataBase{
		Config:          config,
		Params:          params,
		directoryMap:    make(map[string]image.DirectoryMetadata),
		imageMap:        make(map[string]*image.Image),
		addNotifiers:    make(notifiers),
		deleteNotifiers: make(notifiers),
		mkdirNotifiers:  make(makeDirectoryNotifiers),
		deduper:         stringutil.NewStringDeduplicator(false),
	}
	imdb.lockWatcher = lockwatcher.New(&imdb.RWMutex,
		lockwatcher.LockWatcherOptions{
			CheckInterval: config.LockCheckInterval,
			Logger:        prefixlogger.New("ImageServer: ", params.Logger),
			LogTimeout:    config.LockLogTimeout,
		})
	state := concurrent.NewState(0)
	startTime := time.Now()
	var rusageStart, rusageStop syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)
	if err := imdb.scanDirectory(".", state, params.Logger); err != nil {
		return nil, err
	}
	if err := state.Reap(); err != nil {
		return nil, err
	}
	if params.Logger != nil {
		plural := ""
		if imdb.CountImages() != 1 {
			plural = "s"
		}
		syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStop)
		userTime := time.Duration(rusageStop.Utime.Sec)*time.Second +
			time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
			time.Duration(rusageStart.Utime.Sec)*time.Second -
			time.Duration(rusageStart.Utime.Usec)*time.Microsecond
		params.Logger.Printf("Loaded %d image%s in %s (%s user CPUtime)\n",
			imdb.CountImages(), plural, time.Since(startTime), userTime)
		logutil.LogMemory(params.Logger, 0, "after loading")
	}
	return imdb, nil
}

func (imdb *ImageDataBase) scanDirectory(dirname string,
	state *concurrent.State, logger log.DebugLogger) error {
	directoryMetadata, err := imdb.readDirectoryMetadata(dirname)
	if err != nil {
		return err
	}
	imdb.directoryMap[dirname] = directoryMetadata
	file, err := os.Open(path.Join(imdb.BaseDirectory, dirname))
	if err != nil {
		return err
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		if len(name) > 0 && name[0] == '.' {
			continue // Skip hidden paths.
		}
		filename := path.Join(dirname, name)
		var stat syscall.Stat_t
		err := syscall.Lstat(path.Join(imdb.BaseDirectory, filename), &stat)
		if err != nil {
			if err == syscall.ENOENT {
				continue
			}
			return err
		}
		if stat.Mode&syscall.S_IFMT == syscall.S_IFDIR {
			err = imdb.scanDirectory(filename, state, logger)
		} else if stat.Mode&syscall.S_IFMT == syscall.S_IFREG && stat.Size > 0 {
			err = state.GoRun(func() error {
				return imdb.loadFile(filename, logger)
			})
		}
		if err != nil {
			if err == syscall.ENOENT {
				continue
			}
			return err
		}
	}
	return nil
}

func (imdb *ImageDataBase) readDirectoryMetadata(dirname string) (
	image.DirectoryMetadata, error) {
	file, err := os.Open(path.Join(imdb.BaseDirectory, dirname, metadataFile))
	if err != nil {
		if os.IsNotExist(err) {
			return image.DirectoryMetadata{}, nil
		}
		return image.DirectoryMetadata{}, err
	}
	defer file.Close()
	reader := fsutil.NewChecksumReader(file)
	decoder := gob.NewDecoder(reader)
	metadata := image.DirectoryMetadata{}
	if err := decoder.Decode(&metadata); err != nil {
		return image.DirectoryMetadata{}, fmt.Errorf(
			"unable to read directory metadata for \"%s\": %s", dirname, err)
	}
	return metadata, reader.VerifyChecksum()
}

func (imdb *ImageDataBase) loadFile(filename string,
	logger log.DebugLogger) error {
	pathname := path.Join(imdb.BaseDirectory, filename)
	file, err := os.Open(pathname)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := fsutil.NewChecksumReader(file)
	decoder := gob.NewDecoder(reader)
	var img image.Image
	if err := decoder.Decode(&img); err != nil {
		return err
	}
	if err := reader.VerifyChecksum(); err != nil {
		if err == fsutil.ErrorChecksumMismatch {
			logger.Printf("Checksum mismatch for image: %s\n", filename)
			return nil
		}
		if err != io.EOF {
			return err
		}
	}
	if imageIsExpired(&img) {
		imdb.Logger.Printf("Deleting already expired image: %s\n", filename)
		return os.Remove(pathname)
	}
	if err := img.VerifyObjects(imdb.Params.ObjectServer); err != nil {
		if imdb.ReplicationMaster == "" ||
			!strings.Contains(err.Error(), "not available") {
			return fmt.Errorf("error verifying: %s: %s", filename, err)
		}
		err = imdb.fetchMissingObjects(&img,
			prefixlogger.New(filename+": ", logger))
		if err != nil {
			return err
		}
	}
	img.FileSystem.RebuildInodePointers()
	imdb.deduperLock.Lock()
	img.ReplaceStrings(imdb.deduper.DeDuplicate)
	imdb.deduperLock.Unlock()
	if err := img.Verify(); err != nil {
		return err
	}
	if err := imdb.Params.ObjectServer.AdjustRefcounts(true, &img); err != nil {
		return err
	}
	imdb.scheduleExpiration(&img, filename)
	imdb.Lock()
	defer imdb.Unlock()
	imdb.imageMap[filename] = &img
	return nil
}

func (imdb *ImageDataBase) fetchMissingObjects(img *image.Image,
	logger log.DebugLogger) error {
	imdb.objectFetchLock.Lock()
	defer imdb.objectFetchLock.Unlock()
	client, err := srpc.DialHTTP("tcp", imdb.ReplicationMaster, time.Minute)
	if err != nil {
		return err
	}
	defer client.Close()
	objClient := objectclient.AttachObjectClient(client)
	defer objClient.Close()
	return img.GetMissingObjects(imdb.Params.ObjectServer, objClient, logger)
}
