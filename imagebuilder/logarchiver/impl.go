package logarchiver

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type buildLogArchiver struct {
	options           BuildLogArchiveOptions
	params            BuildLogArchiveParams
	fileSizeIncrement uint64
	mutex             sync.Mutex                  // Lock everything below.
	ageList           list.List                   // Oldest first.
	imageStreams      map[string]*imageStreamType // Key: stream name.
	totalSize         uint64
}

type imageStreamType struct {
	images map[string]*imageType // Key: image leaf name.
	name   string
}

type imageType struct {
	ageListElement *list.Element
	buildInfo      BuildInfo
	imageStream    *imageStreamType
	logSize        uint64 // Rounded up.
	modTime        time.Time
	name           string // Leaf name.
}

func roundUp(value, increment uint64) uint64 {
	numBlocks := value / increment
	if numBlocks*increment == value {
		return value
	}
	return (numBlocks + 1) * increment
}

func newBuildLogArchive(options BuildLogArchiveOptions,
	params BuildLogArchiveParams) (*buildLogArchiver, error) {
	archive := &buildLogArchiver{
		imageStreams: make(map[string]*imageStreamType),
		options:      options,
		params:       params,
	}
	if err := archive.computeFileSizeIncrement(); err != nil {
		return nil, fmt.Errorf("error computing file size increment: %s", err)
	}
	startTime := time.Now()
	if err := archive.load(""); err != nil {
		return nil, err
	}
	loadedTime := time.Now()
	archive.makeAgeList()
	sortedTime := time.Now()
	archive.params.Logger.Printf(
		"Loaded build log archive %s in %s, sorted in %s\n",
		format.FormatBytes(archive.totalSize),
		format.Duration(loadedTime.Sub(startTime)),
		format.Duration(sortedTime.Sub(loadedTime)))
	return archive, nil
}

// addEntry adds the image to the image stream and optionally adds it to the
// back of the ageList.
// No lock is taken.
func (a *buildLogArchiver) addEntry(image *imageType, name string,
	addToAgeList bool) {
	streamName := filepath.Dir(name)
	imageStream := a.imageStreams[streamName]
	if imageStream == nil {
		imageStream = &imageStreamType{
			images: make(map[string]*imageType),
			name:   streamName,
		}
		a.imageStreams[streamName] = imageStream
	}
	image.imageStream = imageStream
	imageStream.images[image.name] = image
	a.totalSize += a.imageTotalSize(image)
	if addToAgeList {
		image.ageListElement = a.ageList.PushBack(image)
	}
}

// addEntryWithCheck checks to see if there is sufficient space (deleting old
// entries if needed) and then adds the image to the image stream and the back
// of the ageList.
func (a *buildLogArchiver) addEntryWithCheck(image *imageType,
	name string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for a.imageTotalSize(image)+a.totalSize < a.options.Quota {
		a.addEntry(image, name, true)
		return nil
	}
	targetSize := a.options.Quota * 95 / 100
	if a.imageTotalSize(image)+targetSize > a.options.Quota {
		targetSize -= a.imageTotalSize(image)
	}
	var deletedLogs uint
	origTotalSize := a.totalSize
	for a.totalSize > targetSize {
		oldestElement := a.ageList.Front()
		if err := a.deleteEntry(oldestElement); err != nil {
			return err
		}
		a.ageList.Remove(oldestElement)
		deletedLogs++
	}
	a.params.Logger.Printf("Deleted %d archived build logs consuming %s\n",
		deletedLogs, format.FormatBytes(origTotalSize-a.totalSize))
	a.addEntry(image, name, true)
	return nil
}

func (a *buildLogArchiver) AddBuildLog(imageName string, buildInfo BuildInfo,
	buildLog []byte) error {
	dirname := filepath.Join(a.options.Topdir, imageName)
	if err := os.MkdirAll(filepath.Dir(dirname), fsutil.DirPerms); err != nil {
		return err
	}
	if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
		return err
	}
	doDelete := true
	defer func() {
		if doDelete {
			os.RemoveAll(dirname)
		}
	}()
	err := json.WriteToFile(filepath.Join(dirname, "buildInfo"),
		fsutil.PublicFilePerms, "    ", buildInfo)
	if err != nil {
		return err
	}
	logfile := filepath.Join(dirname, "buildLog")
	err = ioutil.WriteFile(logfile, buildLog, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	image := a.makeEntry(buildInfo, uint64(len(buildLog)), time.Now(),
		imageName)
	if err := a.addEntryWithCheck(image, imageName); err != nil {
		return err
	}
	doDelete = false
	a.params.Logger.Debugf(0, "Archived build log for: %s, %s (%s total)\n",
		imageName, format.FormatBytes(a.imageTotalSize(image)),
		format.FormatBytes(a.totalSize))
	return nil
}

func (a *buildLogArchiver) computeFileSizeIncrement() error {
	if err := os.MkdirAll(a.options.Topdir, fsutil.DirPerms); err != nil {
		return err
	}
	file, err := ioutil.TempFile(a.options.Topdir, "******")
	if err != nil {
		return err
	}
	filename := file.Name()
	defer os.Remove(filename)
	if _, err := file.Write([]byte{'\n'}); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	var statbuf wsyscall.Stat_t
	if err := wsyscall.Stat(filename, &statbuf); err != nil {
		return err
	}
	if statbuf.Blocks < 1 {
		statbuf.Blocks = 1
	}
	a.fileSizeIncrement = uint64(statbuf.Blocks) * 512
	return nil
}

func (a *buildLogArchiver) deleteEntry(element *list.Element) error {
	image := element.Value.(*imageType)
	imageStream := image.imageStream
	dirname := filepath.Join(a.options.Topdir, imageStream.name, image.name)
	if err := os.RemoveAll(dirname); err != nil {
		return err
	}
	delete(imageStream.images, image.name)
	a.totalSize -= a.imageTotalSize(image)
	return nil
}

func (a *buildLogArchiver) imageTotalSize(image *imageType) uint64 {
	return image.logSize + a.fileSizeIncrement
}

func (a *buildLogArchiver) load(dirname string) error {
	dirpath := filepath.Join(a.options.Topdir, dirname)
	names, err := fsutil.ReadDirnames(dirpath, false)
	if err != nil {
		return err
	}
	var buildInfoPathname, buildLogPathname string
	for _, name := range names {
		switch name {
		case "buildInfo":
			buildInfoPathname = filepath.Join(dirpath, name)
			continue
		case "buildLog":
			buildLogPathname = filepath.Join(dirpath, name)
			continue
		}
		if err := a.load(filepath.Join(dirname, name)); err != nil {
			return err
		}
	}
	if buildLogPathname == "" {
		return nil
	}
	var buildInfo BuildInfo
	if buildInfoPathname != "" {
		if err := json.ReadFromFile(buildInfoPathname, &buildInfo); err != nil {
			return err
		}
	}
	if fi, err := os.Stat(buildLogPathname); err != nil {
		return err
	} else {
		image := a.makeEntry(buildInfo, uint64(fi.Size()), fi.ModTime(),
			dirname)
		a.addEntry(image, dirname, false)
	}
	return nil
}

func (a *buildLogArchiver) makeAgeList() {
	var imageList []*imageType
	for _, imageStream := range a.imageStreams {
		for _, image := range imageStream.images {
			imageList = append(imageList, image)
		}
	}
	// Sort so that oldest mtime is the first slice entry.
	sort.Slice(imageList, func(i, j int) bool {
		return imageList[i].modTime.Before(imageList[j].modTime)
	})
	for _, image := range imageList {
		image.ageListElement = a.ageList.PushBack(image)
	}
}

func (a *buildLogArchiver) makeEntry(buildInfo BuildInfo, logSize uint64,
	modTime time.Time, name string) *imageType {
	image := &imageType{
		buildInfo: buildInfo,
		logSize:   roundUp(logSize, a.fileSizeIncrement),
		modTime:   modTime,
		name:      filepath.Base(name),
	}
	return image
}
