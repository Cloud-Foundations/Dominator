package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	imageclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/image/packageutil"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

const timeFormat = "2006-01-02:15:04:05"

var (
	errorTestTimedOut = errors.New("test timed out")
	tmpFilter         *filter.Filter
)

type hasher struct {
	cache *treeCache
	objQ  *objectclient.ObjectAdderQueue
}

func init() {
	if filt, err := filter.New([]string{"/tmp/.*"}); err != nil {
		panic(err)
	} else {
		tmpFilter = filt
	}
}

func (h *hasher) Hash(reader io.Reader, length uint64) (
	hash.Hash, error) {
	hash, err := h.objQ.Add(reader, length)
	if err != nil {
		return hash, errors.New("error sending image data: " + err.Error())
	}
	return hash, nil
}

func (h *hasher) OpenAndHash(inode *filesystem.RegularInode,
	pathName string) (bool, error) {
	if len(h.cache.inodeTable) < 1 {
		return false, nil
	}
	inum, ok := h.cache.pathToInode[pathName]
	if !ok {
		return false, nil
	}
	inodeData, ok := h.cache.inodeTable[inum]
	if !ok {
		return false, nil
	}
	if inode.Size != inodeData.size {
		return false, nil
	}
	var stat syscall.Stat_t
	if err := syscall.Stat(pathName, &stat); err != nil {
		return false, err
	}
	if stat.Ino != inum {
		return false, nil
	}
	if stat.Size != int64(inodeData.size) {
		return false, nil
	}
	if stat.Ctim != inodeData.ctime {
		return false, nil
	}
	inode.Hash = inodeData.hash
	h.cache.numHits++
	h.cache.hitBytes += inodeData.size
	return true, nil
}

func addImage(client *srpc.Client, request proto.BuildImageRequest,
	img *image.Image) (string, error) {
	if request.ExpiresIn > 0 {
		img.ExpiresAt = time.Now().Add(request.ExpiresIn)
	}
	name := makeImageName(request.StreamName)
	if err := imageclient.AddImage(client, name, img); err != nil {
		return "", errors.New("remote error: " + err.Error())
	}
	return name, nil
}

func buildFileSystem(client *srpc.Client, dirname string,
	scanFilter *filter.Filter, cache *treeCache) (
	*filesystem.FileSystem, error) {
	h := hasher{cache: cache}
	var err error
	h.objQ, err = objectclient.NewObjectAdderQueue(client)
	if err != nil {
		return nil, err
	}
	fs, err := buildFileSystemWithHasher(dirname, &h, scanFilter)
	if err != nil {
		h.objQ.Close()
		return nil, err
	}
	err = h.objQ.Close()
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func buildFileSystemWithHasher(dirname string, h *hasher,
	scanFilter *filter.Filter) (
	*filesystem.FileSystem, error) {
	fs, err := scanner.ScanFileSystem(dirname, nil, scanFilter, nil, h, nil)
	if err != nil {
		return nil, err
	}
	return &fs.FileSystem, nil
}

func listPackages(g *goroutine.Goroutine, rootDir string) (
	[]image.Package, error) {
	return packageutil.GetPackageList(func(cmd string, w io.Writer) error {
		return runInTarget(g, nil, w, rootDir, nil, packagerPathname, cmd)
	})
}

func makeImageName(streamName string) string {
	return path.Join(streamName, time.Now().Format(timeFormat))
}

func packImage(g *goroutine.Goroutine, client *srpc.Client,
	request proto.BuildImageRequest, dirname string, scanFilter *filter.Filter,
	cache *treeCache, computedFilesList []util.ComputedFile,
	imageFilter *filter.Filter, trig *triggers.Triggers,
	buildLog buildLogger) (*image.Image, error) {
	if cache == nil {
		cache = &treeCache{}
	}
	if g == nil {
		var err error
		g, err = newNamespaceTarget()
		if err != nil {
			return nil, err
		}
		defer g.Quit()
	}
	packages, err := listPackages(g, dirname)
	if err != nil {
		return nil, fmt.Errorf("error listing packages: %s", err)
	}
	if err := util.DeletedFilteredFiles(dirname, tmpFilter); err != nil {
		return nil, err
	}
	buildStartTime := time.Now()
	fs, err := buildFileSystem(client, dirname, scanFilter, cache)
	if err != nil {
		return nil, fmt.Errorf("error building file-system: %s", err)
	}
	if err := util.SpliceComputedFiles(fs, computedFilesList); err != nil {
		return nil, fmt.Errorf("error splicing computed files: %s", err)
	}
	fs.ComputeTotalDataBytes()
	duration := time.Since(buildStartTime)
	speed := uint64(float64(fs.TotalDataBytes-cache.hitBytes) /
		duration.Seconds())
	fmt.Fprintf(buildLog, "Skipped %d unchanged objects (%s)\n",
		cache.numHits, format.FormatBytes(cache.hitBytes))
	fmt.Fprintf(buildLog,
		"Scanned file-system and uploaded %d objects (%s) in %s (%s/s)\n",
		fs.NumRegularInodes-cache.numHits,
		format.FormatBytes(fs.TotalDataBytes-cache.hitBytes),
		format.Duration(duration), format.FormatBytes(speed))
	_, oldImage, err := getLatestImage(client, request.StreamName, buildLog)
	if err != nil {
		return nil, fmt.Errorf("error getting latest image: %s", err)
	} else if oldImage != nil {
		patchStartTime := time.Now()
		util.CopyMtimes(oldImage.FileSystem, fs)
		fmt.Fprintf(buildLog, "Copied mtimes in %s\n",
			format.Duration(time.Since(patchStartTime)))
	}
	if err := runTests(g, dirname, buildLog); err != nil {
		return nil, err
	}
	objClient := objectclient.AttachObjectClient(client)
	// Make a copy of the build log because AddObject() drains the buffer.
	logReader := bytes.NewBuffer(buildLog.Bytes())
	hashVal, _, err := objClient.AddObject(logReader, uint64(logReader.Len()),
		nil)
	if err != nil {
		return nil, err
	}
	if err := objClient.Close(); err != nil {
		return nil, err
	}
	img := &image.Image{
		BuildLog:   &image.Annotation{Object: &hashVal},
		FileSystem: fs,
		Filter:     imageFilter,
		Triggers:   trig,
		Packages:   packages,
	}
	if err := img.Verify(); err != nil {
		return nil, err
	}
	return img, nil
}

func runTests(g *goroutine.Goroutine, rootDir string,
	buildLog buildLogger) error {
	var testProgrammes []string
	err := filepath.Walk(filepath.Join(rootDir, "tests"),
		func(path string, fi os.FileInfo, err error) error {
			if fi == nil || !fi.Mode().IsRegular() || fi.Mode()&0100 == 0 {
				return nil
			}
			testProgrammes = append(testProgrammes, path[len(rootDir):])
			return nil
		})
	if err != nil {
		return err
	}
	if len(testProgrammes) < 1 {
		return nil
	}
	fmt.Fprintf(buildLog, "Running %d tests\n", len(testProgrammes))
	results := make(chan testResultType, 1)
	for _, prog := range testProgrammes {
		go func(prog string) {
			results <- runTest(g, rootDir, prog)
		}(prog)
	}
	numFailures := 0
	for range testProgrammes {
		result := <-results
		io.Copy(buildLog, &result)
		if result.err != nil {
			fmt.Fprintf(buildLog, "error running: %s: %s\n",
				result.prog, result.err)
			numFailures++
		} else {
			fmt.Fprintf(buildLog, "%s passed in %s\n",
				result.prog, format.Duration(result.duration))
		}
		fmt.Fprintln(buildLog)
	}
	if numFailures > 0 {
		return fmt.Errorf("%d tests failed", numFailures)
	}
	return nil
}

func runTest(g *goroutine.Goroutine, rootDir, prog string) testResultType {
	startTime := time.Now()
	result := testResultType{
		buffer: make(chan byte, 4096),
		prog:   prog,
	}
	errChannel := make(chan error, 1)
	timer := time.NewTimer(time.Second * 10)
	go func() {
		errChannel <- runInTarget(g, nil, &result, rootDir, nil,
			packagerPathname, "run", prog)
	}()
	select {
	case result.err = <-errChannel:
		result.duration = time.Since(startTime)
	case <-timer.C:
		result.err = errorTestTimedOut
	}
	return result
}

func (w *testResultType) Read(p []byte) (int, error) {
	for count := 0; count < len(p); count++ {
		select {
		case p[count] = <-w.buffer:
		default:
			return count, io.EOF
		}
	}
	return len(p), nil
}

func (w *testResultType) Write(p []byte) (int, error) {
	for index, ch := range p {
		select {
		case w.buffer <- ch:
		default:
			return index, io.ErrShortWrite
		}
	}
	return len(p), nil
}
