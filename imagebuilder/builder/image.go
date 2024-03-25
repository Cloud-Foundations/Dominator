package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	stdlog "log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/gitutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

type gitInfoType struct {
	branch   string
	commitId string
	gitUrl   string
}

func (stream *imageStreamType) build(b *Builder, client srpc.ClientI,
	request proto.BuildImageRequest, buildLog buildLogger) (
	*image.Image, error) {
	manifestDirectory, gitInfo, err := stream.getManifest(b, request.StreamName,
		request.GitBranch, request.Variables, buildLog)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(manifestDirectory)
	img, err := buildImageFromManifest(client, manifestDirectory, request,
		b.bindMounts, stream, gitInfo, b.mtimesCopyFilter, buildLog)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (stream *imageStreamType) getenv() map[string]string {
	envTable := make(map[string]string, len(stream.Variables)+3)
	for key, value := range stream.Variables {
		envTable[key] = expandExpression(value, func(name string) string {
			if name == "IMAGE_STREAM" {
				return stream.name
			}
			return ""
		})
	}
	envTable["IMAGE_STREAM"] = stream.name
	envTable["IMAGE_STREAM_DIRECTORY_NAME"] = filepath.Dir(stream.name)
	envTable["IMAGE_STREAM_LEAF_NAME"] = filepath.Base(stream.name)
	return envTable
}

// getManifestLocation will expand variables and return the actual manifest
// location. These data may include secrets (i.e. username and password).
// If b is nil then secret variables are not expanded and thus the returned
// data do not contain secrets but may be incorrect.
func (stream *imageStreamType) getManifestLocation(b *Builder,
	variables map[string]string) manifestLocationType {
	var variableFunc func(string) string
	if b == nil {
		variableFunc = func(name string) string {
			return stream.getenv()[name]
		}
	} else {
		variableFunc = b.getVariableFunc(stream.getenv(), variables)
	}
	return manifestLocationType{
		directory: expandExpression(stream.ManifestDirectory, variableFunc),
		url:       expandExpression(stream.ManifestUrl, variableFunc),
	}
}

func (stream *imageStreamType) getManifest(b *Builder, streamName string,
	gitBranch string, variables map[string]string,
	buildLog io.Writer) (string, *gitInfoType, error) {
	if gitBranch == "" {
		gitBranch = "master"
	}
	manifestRoot, err := makeTempDirectory("",
		strings.Replace(streamName, "/", "_", -1)+".manifest")
	if err != nil {
		return "", nil, err
	}
	doCleanup := true
	defer func() {
		if doCleanup {
			os.RemoveAll(manifestRoot)
		}
	}()
	manifestLocation := stream.getManifestLocation(b, variables)
	if rootDir, err := urlToLocal(manifestLocation.url); err != nil {
		return "", nil, err
	} else if rootDir != "" {
		if gitBranch != "master" {
			return "", nil,
				fmt.Errorf("branch: %s is not master", gitBranch)
		}
		sourceTree := filepath.Join(rootDir, manifestLocation.directory)
		fmt.Fprintf(buildLog, "Copying manifest tree: %s\n", sourceTree)
		if err := fsutil.CopyTree(manifestRoot, sourceTree); err != nil {
			return "", nil, fmt.Errorf("error copying manifest: %s", err)
		}
		doCleanup = false
		return manifestRoot, nil, nil
	}
	var patterns []string
	if manifestLocation.directory != "" {
		patterns = append(patterns, manifestLocation.directory+"/*")
	}
	err = gitShallowClone(manifestRoot, manifestLocation.url,
		stream.ManifestUrl, gitBranch, patterns, buildLog)
	if err != nil {
		return "", nil, err
	}
	gitDirectory := filepath.Join(manifestRoot, ".git")
	var gitInfo *gitInfoType
	// The specified branch/tag/commit will be in the "master" branch in the
	// cloned repository.
	commitId, err := gitutil.GetCommitIdOfRef(manifestRoot, "HEAD")
	if err != nil {
		return "", nil, err
	} else {
		gitInfo = &gitInfoType{
			branch:   gitBranch,
			commitId: commitId,
			gitUrl:   manifestLocation.url,
		}
	}
	if err := os.RemoveAll(gitDirectory); err != nil {
		return "", nil, err
	}
	if manifestLocation.directory != "" {
		// Move manifest directory into manifestRoot, remove anything else.
		err := os.Rename(filepath.Join(manifestRoot,
			manifestLocation.directory),
			gitDirectory)
		if err != nil {
			return "", nil, err
		}
		filenames, err := listDirectory(manifestRoot)
		if err != nil {
			return "", nil, err
		}
		for _, filename := range filenames {
			if filename == ".git" {
				continue
			}
			err := os.RemoveAll(filepath.Join(manifestRoot, filename))
			if err != nil {
				return "", nil, err
			}
		}
		filenames, err = listDirectory(gitDirectory)
		if err != nil {
			return "", nil, err
		}
		for _, filename := range filenames {
			err := os.Rename(filepath.Join(gitDirectory, filename),
				filepath.Join(manifestRoot, filename))
			if err != nil {
				return "", nil, err
			}
		}
		if err := os.Remove(gitDirectory); err != nil {
			return "", nil, err
		}
	}
	doCleanup = false
	return manifestRoot, gitInfo, nil
}

func (stream *imageStreamType) getSourceImage(b *Builder, buildLog io.Writer) (
	string, string, *gitInfoType, []byte, *manifestConfigType, error) {
	manifestDirectory, gitInfo, err := stream.getManifest(stream.builder,
		stream.name, "", nil, buildLog)
	if err != nil {
		return "", "", nil, nil, nil, err
	}
	doRemove := true
	defer func() {
		if doRemove {
			os.RemoveAll(manifestDirectory)
		}
	}()
	manifestFilename := filepath.Join(manifestDirectory, "manifest")
	manifestBytes, err := ioutil.ReadFile(manifestFilename)
	if err != nil {
		return "", "", nil, nil, nil, err
	}
	var manifest manifestConfigType
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", "", nil, nil, nil, err
	}
	sourceImageName := expandExpression(manifest.SourceImage,
		func(name string) string {
			return stream.getenv()[name]
		})
	doRemove = false
	return manifestDirectory, sourceImageName, gitInfo, manifestBytes,
		&manifest, nil
}

func listDirectory(directoryName string) ([]string, error) {
	directory, err := os.Open(directoryName)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	filenames, err := directory.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return filenames, nil
}

func runCommand(buildLog io.Writer, cwd string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdout = buildLog
	cmd.Stderr = buildLog
	return cmd.Run()
}

func buildImageFromManifest(client srpc.ClientI, manifestDir string,
	request proto.BuildImageRequest, bindMounts []string,
	envGetter environmentGetter, gitInfo *gitInfoType,
	mtimesCopyFilter *filter.Filter, buildLog buildLogger) (
	*image.Image, error) {
	// First load all the various manifest files (fail early on error).
	computedFilesList, addComputedFiles, err := loadComputedFiles(manifestDir)
	if err != nil {
		return nil, err
	}
	imageFilter, addFilter, err := loadFilter(manifestDir)
	if err != nil {
		return nil, err
	}
	tgs, err := loadTags(manifestDir)
	if err != nil {
		return nil, err
	}
	imageTriggers, addTriggers, err := loadTriggers(manifestDir)
	if err != nil {
		return nil, err
	}
	rootDir, err := makeTempDirectory("",
		strings.Replace(request.StreamName, "/", "_", -1)+".root")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(rootDir)
	fmt.Fprintf(buildLog, "Created image working directory: %s\n", rootDir)
	vGetter := variablesGetter(envGetter.getenv()).copy()
	vGetter.merge(request.Variables)
	if gitInfo != nil {
		vGetter.add("MANIFEST_GIT_COMMIT_ID", gitInfo.commitId)
	}
	vGetter.add("REQUESTED_GIT_BRANCH", request.GitBranch)
	request.Variables = vGetter
	manifest, err := unpackImageAndProcessManifest(client, manifestDir,
		request.MaxSourceAge, rootDir, bindMounts, false, vGetter, buildLog)
	if err != nil {
		return nil, err
	}
	ctimeResolution, err := getCtimeResolution()
	if err != nil {
		return nil, err
	}
	time.Sleep(ctimeResolution)
	fmt.Fprintf(buildLog, "Waited %s (Ctime resolution)\n",
		format.Duration(ctimeResolution))
	if fi, err := os.Lstat(filepath.Join(manifestDir, "tests")); err == nil {
		if fi.IsDir() {
			testsDir := filepath.Join(rootDir, "tests", request.StreamName)
			if err := os.MkdirAll(testsDir, fsutil.DirPerms); err != nil {
				return nil, err
			}
			err := copyFiles(manifestDir, "tests", testsDir, buildLog)
			if err != nil {
				return nil, err
			}
		}
	}
	if addComputedFiles {
		computedFilesList = util.MergeComputedFiles(
			manifest.sourceImageInfo.computedFiles, computedFilesList)
	}
	if addFilter {
		mergeableFilter := &filter.MergeableFilter{}
		mergeableFilter.Merge(manifest.sourceImageInfo.filter)
		mergeableFilter.Merge(imageFilter)
		imageFilter = mergeableFilter.ExportFilter()
	}
	if addTriggers {
		mergeableTriggers := &triggers.MergeableTriggers{}
		mergeableTriggers.Merge(manifest.sourceImageInfo.triggers)
		mergeableTriggers.Merge(imageTriggers)
		imageTriggers = mergeableTriggers.ExportTriggers()
	}
	if manifest.mtimesCopyFilter != nil {
		mtimesCopyFilter = manifest.mtimesCopyFilter
	} else if manifest.mtimesCopyAddFilter != nil {
		mf := &filter.MergeableFilter{}
		mf.Merge(mtimesCopyFilter)
		mf.Merge(manifest.mtimesCopyAddFilter)
		mtimesCopyFilter = mf.ExportFilter()
	}
	img, err := packImage(nil, client, request, rootDir, manifest.filter,
		manifest.sourceImageInfo.treeCache, computedFilesList, imageFilter,
		tgs, imageTriggers, mtimesCopyFilter, buildLog)
	if err != nil {
		return nil, err
	}
	if gitInfo != nil {
		img.BuildBranch = gitInfo.branch
		img.BuildCommitId = gitInfo.commitId
		img.BuildGitUrl = gitInfo.gitUrl
	}
	img.SourceImage = manifest.sourceImageInfo.imageName
	return img, nil
}

func buildImageFromManifestAndUpload(client srpc.ClientI,
	options BuildLocalOptions, streamName string, expiresIn time.Duration,
	buildLog buildLogger) (*image.Image, string, error) {
	request := proto.BuildImageRequest{
		StreamName: streamName,
		ExpiresIn:  expiresIn,
	}
	img, err := buildImageFromManifest(
		client,
		options.ManifestDirectory,
		request,
		options.BindMounts,
		&imageStreamType{
			name:      streamName,
			Variables: options.Variables,
		},
		nil,
		options.MtimesCopyFilter,
		buildLog)
	if err != nil {
		return nil, "", err
	}
	name, err := addImage(client, request, img)
	if err != nil {
		return nil, "", err
	}
	return img, name, nil
}

func buildTreeCache(rootDir string, fs *filesystem.FileSystem,
	buildLog io.Writer) (*treeCache, error) {
	cache := treeCache{
		inodeTable:  make(map[uint64]inodeData),
		pathToInode: make(map[string]uint64),
	}
	filenameToInodeTable := fs.FilenameToInodeTable()
	rootLength := len(rootDir)
	startTime := time.Now()
	err := filepath.Walk(rootDir,
		func(path string, info os.FileInfo, err error) error {
			if info.Mode()&os.ModeType != 0 {
				return nil
			}
			rootedPath := path[rootLength:]
			inum, ok := filenameToInodeTable[rootedPath]
			if !ok {
				return nil
			}
			gInode, ok := fs.InodeTable[inum]
			if !ok {
				return nil
			}
			rInode, ok := gInode.(*filesystem.RegularInode)
			if !ok {
				return nil
			}
			var stat syscall.Stat_t
			if err := syscall.Stat(path, &stat); err != nil {
				return err
			}
			cache.inodeTable[stat.Ino] = inodeData{
				ctime: stat.Ctim,
				hash:  rInode.Hash,
				size:  uint64(stat.Size),
			}
			cache.pathToInode[path] = uint64(stat.Ino)
			return nil
		})
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(buildLog, "Built tree cache in: %s\n",
		format.Duration(time.Since(startTime)))
	return &cache, nil
}

func buildTreeFromManifest(client srpc.ClientI, options BuildLocalOptions,
	buildLog io.Writer) (string, error) {
	rootDir, err := makeTempDirectory("", "tree")
	if err != nil {
		return "", err
	}
	_, err = unpackImageAndProcessManifest(client,
		options.ManifestDirectory, 0, rootDir, options.BindMounts, true,
		variablesGetter(options.Variables), buildLog)
	if err != nil {
		os.RemoveAll(rootDir)
		return "", err
	}
	return rootDir, nil
}

func listComputedFiles(fs *filesystem.FileSystem) []util.ComputedFile {
	var computedFiles []util.ComputedFile
	fs.ForEachFile(
		func(path string, _ uint64, inode filesystem.GenericInode) error {
			if inode, ok := inode.(*filesystem.ComputedRegularInode); ok {
				computedFiles = append(computedFiles, util.ComputedFile{
					Filename: path,
					Source:   inode.Source,
				})
			}
			return nil
		})
	return computedFiles
}

func loadComputedFiles(manifestDir string) ([]util.ComputedFile, bool, error) {
	computedFiles, err := util.LoadComputedFiles(filepath.Join(manifestDir,
		"computed-files.json"))
	if os.IsNotExist(err) {
		computedFiles, err = util.LoadComputedFiles(
			filepath.Join(manifestDir, "computed-files"))
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	haveComputedFiles := err == nil
	addComputedFiles, err := util.LoadComputedFiles(
		filepath.Join(manifestDir, "computed-files.add.json"))
	if os.IsNotExist(err) {
		addComputedFiles, err = util.LoadComputedFiles(
			filepath.Join(manifestDir, "computed-files.add"))
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	haveAddComputedFiles := err == nil
	if !haveComputedFiles && !haveAddComputedFiles {
		return nil, false, nil
	} else if haveComputedFiles && haveAddComputedFiles {
		return nil, false, errors.New(
			"computed-files and computed-files.add files both present")
	} else if haveComputedFiles {
		return computedFiles, false, nil
	} else {
		return addComputedFiles, true, nil
	}
}

func loadFilter(manifestDir string) (*filter.Filter, bool, error) {
	imageFilter, err := filter.Load(filepath.Join(manifestDir, "filter"))
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	addFilter, err := filter.Load(filepath.Join(manifestDir, "filter.add"))
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	if imageFilter == nil && addFilter == nil {
		return nil, false, nil
	} else if imageFilter != nil && addFilter != nil {
		return nil, false, errors.New(
			"filter and filter.add files both present")
	} else if imageFilter != nil {
		return imageFilter, false, nil
	} else {
		return addFilter, true, nil
	}
}

func loadTags(manifestDir string) (tags.Tags, error) {
	var tgs tags.Tags
	err := libjson.ReadFromFile(filepath.Join(manifestDir, "tags.json"), &tgs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(tgs) < 1 {
		return nil, nil
	}
	return tgs, nil
}

func loadTriggers(manifestDir string) (*triggers.Triggers, bool, error) {
	imageTriggers, err := triggers.Load(filepath.Join(manifestDir, "triggers"))
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	addTriggers, err := triggers.Load(filepath.Join(manifestDir,
		"triggers.add"))
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	if imageTriggers == nil && addTriggers == nil {
		return nil, false, nil
	} else if imageTriggers != nil && addTriggers != nil {
		return nil, false, errors.New(
			"triggers and triggers.add files both present")
	} else if imageTriggers != nil {
		return imageTriggers, false, nil
	} else {
		return addTriggers, true, nil
	}
}

func unpackImage(client srpc.ClientI, streamName, buildCommitId string,
	sourceImageTagsToMatch tags.MatchTags, maxSourceAge time.Duration,
	rootDir string, buildLog io.Writer) (*sourceImageInfoType, error) {
	ctimeResolution, err := getCtimeResolution()
	if err != nil {
		return nil, err
	}
	imageName, sourceImage, err := getLatestImage(client, streamName,
		buildCommitId, sourceImageTagsToMatch, buildLog)
	if err != nil {
		return nil, err
	}
	var specifiedStream string
	if buildCommitId == "" {
		specifiedStream = streamName
	} else {
		specifiedStream = streamName + "@gitCommitId:" + buildCommitId
	}
	if len(sourceImageTagsToMatch) > 0 {
		specifiedStream += fmt.Sprintf("@tags:%v", sourceImageTagsToMatch)
	}
	if sourceImage == nil {
		return nil, &buildErrorType{
			error:                  "no source image: " + specifiedStream,
			needSourceImage:        true,
			sourceImage:            streamName,
			sourceImageGitCommitId: buildCommitId,
		}
	}
	if maxSourceAge > 0 && time.Since(sourceImage.CreatedOn) > maxSourceAge {
		return nil, &buildErrorType{
			error:                  "too old source image: " + specifiedStream,
			needSourceImage:        true,
			sourceImage:            streamName,
			sourceImageGitCommitId: buildCommitId,
		}
	}
	objClient := objectclient.AttachObjectClient(client)
	defer objClient.Close()
	err = util.Unpack(sourceImage.FileSystem, objClient, rootDir,
		stdlog.New(buildLog, "", 0))
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(buildLog, "Source image: %s\n", imageName)
	treeCache, err := buildTreeCache(rootDir, sourceImage.FileSystem, buildLog)
	if err != nil {
		return nil, err
	}
	time.Sleep(ctimeResolution)
	fmt.Fprintf(buildLog, "Waited %s (Ctime resolution)\n",
		format.Duration(ctimeResolution))
	return &sourceImageInfoType{
		computedFiles: listComputedFiles(sourceImage.FileSystem),
		filter:        sourceImage.Filter,
		imageName:     imageName,
		treeCache:     treeCache,
		triggers:      sourceImage.Triggers,
	}, nil
}

func urlToLocal(urlValue string) (string, error) {
	if parsedUrl, err := url.Parse(urlValue); err == nil {
		if parsedUrl.Scheme == "dir" {
			if parsedUrl.Path[0] != '/' {
				return "", fmt.Errorf("missing leading slash: %s",
					parsedUrl.Path)
			}
			return parsedUrl.Path, nil
		}
	}
	return "", nil
}
