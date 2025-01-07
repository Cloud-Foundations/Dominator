package builder

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/expand"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

func deleteDirectories(directoriesToDelete []string) error {
	for index := len(directoriesToDelete) - 1; index >= 0; index-- {
		if err := os.Remove(directoriesToDelete[index]); err != nil {
			return err
		}
	}
	return nil
}

func makeDirectory(directory string, directoriesToDelete []string,
	directoriesWhichExist map[string]struct{},
	bindMountDirectories map[string]struct{},
	buildLog io.Writer) ([]string, error) {
	if _, ok := directoriesWhichExist[directory]; ok {
		return directoriesToDelete, nil
	} else if fi, err := os.Stat(directory); err != nil {
		if !os.IsNotExist(err) {
			return directoriesToDelete, err
		}
		var err error
		directoriesToDelete, err = makeDirectory(filepath.Dir(directory),
			directoriesToDelete, directoriesWhichExist, bindMountDirectories,
			buildLog)
		if err != nil {
			return directoriesToDelete, err
		}
		if _, ok := bindMountDirectories[directory]; ok {
			fmt.Fprintf(buildLog, "Making bind mount point: %s\n", directory)
		} else {
			fmt.Fprintf(buildLog,
				"Making intermediate directory for bind mount: %s\n",
				directory)
		}
		if err := os.Mkdir(directory, fsutil.DirPerms); err != nil {
			return nil, err
		}
		directoriesToDelete = append(directoriesToDelete, directory)
		directoriesWhichExist[directory] = struct{}{}
		return directoriesToDelete, nil
	} else if !fi.IsDir() {
		return directoriesToDelete,
			fmt.Errorf("%s is not a directory", directory)
	} else {
		directoriesWhichExist[directory] = struct{}{}
		return directoriesToDelete, nil
	}
}

func makeMountPoints(rootDir string, bindMounts []string,
	buildLog io.Writer) ([]string, error) {
	var directoriesToDelete []string
	directoriesWhichExist := make(map[string]struct{})
	defer deleteDirectories(directoriesToDelete)
	bindMountDirectories := make(map[string]struct{}, len(bindMounts))
	for _, bindMount := range bindMounts {
		bindMountDirectories[filepath.Join(rootDir, bindMount)] = struct{}{}
	}
	for _, bindMount := range bindMounts {
		directory := filepath.Join(rootDir, bindMount)
		var err error
		directoriesToDelete, err = makeDirectory(directory, directoriesToDelete,
			directoriesWhichExist, bindMountDirectories, buildLog)
		if err != nil {
			return nil, err
		}
	}
	retval := directoriesToDelete
	directoriesToDelete = nil // Do not clean up in the defer.
	return retval, nil
}

// readManifestFile will read the manifest file in the manifest directory and
// will apply variable expansion to the source image name using envGetter if not
// nil.
func readManifestFile(manifestDir string, envGetter environmentGetter) (
	manifestConfigType, error) {
	manifestFile := filepath.Join(manifestDir, "manifest")
	var manifestConfig manifestConfigType
	if err := json.ReadFromFile(manifestFile, &manifestConfig); err != nil {
		return manifestConfigType{},
			errors.New("error reading manifest file: " + err.Error())
	}
	if envGetter == nil {
		return manifestConfig, nil
	}
	manifestConfig.SourceImage = expand.Expression(manifestConfig.SourceImage,
		func(name string) string {
			return envGetter.getenv()[name]
		})
	for key, value := range manifestConfig.SourceImageBuildVariables {
		newValue := expand.Expression(value, func(name string) string {
			return envGetter.getenv()[name]
		})
		manifestConfig.SourceImageBuildVariables[key] = newValue
	}
	manifestConfig.SourceImageGitCommitId = expand.Expression(
		manifestConfig.SourceImageGitCommitId,
		func(name string) string {
			return envGetter.getenv()[name]
		})
	for _, values := range manifestConfig.SourceImageTagsToMatch {
		for index, value := range values {
			newValue := expand.Expression(value, func(name string) string {
				return envGetter.getenv()[name]
			})
			values[index] = newValue
		}
	}
	return manifestConfig, nil
}

func unpackImageAndProcessManifest(client srpc.ClientI, manifestDir string,
	maxSourceAge time.Duration, rootDir string, bindMounts []string,
	applyFilter bool, envGetter environmentGetter,
	buildLog io.Writer) (manifestType, error) {
	manifestConfig, err := readManifestFile(manifestDir, envGetter)
	if err != nil {
		return manifestType{}, err
	}
	var mtimesCopyAddFilter, mtimesCopyFilter *filter.Filter
	if len(manifestConfig.MtimesCopyAddFilterLines) > 0 {
		mtimesCopyAddFilter, err = filter.New(
			manifestConfig.MtimesCopyAddFilterLines)
		if err != nil {
			return manifestType{}, err
		}
	}
	if len(manifestConfig.MtimesCopyFilterLines) > 0 {
		mtimesCopyFilter, err = filter.New(manifestConfig.MtimesCopyFilterLines)
		if err != nil {
			return manifestType{}, err
		}
	}
	sourceImageInfo, err := unpackImage(client, manifestConfig.SourceImage,
		manifestConfig.SourceImageGitCommitId,
		manifestConfig.SourceImageTagsToMatch,
		maxSourceAge, rootDir, buildLog)
	if err != nil {
		var buildError *BuildErrorType
		if errors.As(err, &buildError) {
			buildError.SourceImageBuildVariables =
				manifestConfig.SourceImageBuildVariables
		}
		return manifestType{}, fmt.Errorf("error unpacking image: %w", err)
	}
	startTime := time.Now()
	err = processManifest(manifestDir, rootDir, bindMounts, envGetter, buildLog)
	if err != nil {
		return manifestType{},
			errors.New("error processing manifest: " + err.Error())
	}
	if applyFilter && manifestConfig.Filter != nil {
		err := util.DeleteFilteredFiles(rootDir, manifestConfig.Filter)
		if err != nil {
			return manifestType{}, err
		}
	}
	fmt.Fprintf(buildLog, "Processed manifest in %s\n",
		format.Duration(time.Since(startTime)))
	return manifestType{
		filter:              manifestConfig.Filter,
		mtimesCopyAddFilter: mtimesCopyAddFilter,
		mtimesCopyFilter:    mtimesCopyFilter,
		sourceImageInfo:     sourceImageInfo,
	}, nil
}

func processManifest(manifestDir, rootDir string, bindMounts []string,
	envGetter environmentGetter, buildLog io.Writer) error {
	// Copy in system /etc/resolv.conf
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return err
	}
	defer file.Close()
	for index, bindMount := range bindMounts {
		bindMounts[index] = filepath.Clean(bindMount)
	}
	directoriesToDelete, err := makeMountPoints(rootDir, bindMounts, buildLog)
	if err != nil {
		return err
	}
	defer func() { // Need to evaluate directoriesToDelete in deferred func.
		deleteDirectories(directoriesToDelete)
	}()
	g, err := newNamespaceTargetWithMounts(rootDir, bindMounts)
	if err != nil {
		return err
	}
	defer g.Quit()
	err = runInTarget(g, file, buildLog, rootDir, envGetter,
		packagerPathname, "copy-in", "/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("error copying in /etc/resolv.conf: %s", err)
	}
	if err := copyFiles(manifestDir, "files", rootDir, buildLog); err != nil {
		return err
	}
	err = runScripts(g, manifestDir, "pre-install-scripts", rootDir, envGetter,
		buildLog)
	if err != nil {
		return err
	}
	packageList, err := fsutil.LoadLines(filepath.Join(manifestDir,
		"package-list"))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	if len(packageList) > 0 {
		err := updatePackageDatabase(g, rootDir, envGetter, buildLog)
		if err != nil {
			return err
		}
	}
	err = installPackages(g, packageList, rootDir, envGetter, buildLog)
	if err != nil {
		return errors.New("error installing packages: " + err.Error())
	}
	err = copyFiles(manifestDir, "post-install-files", rootDir, buildLog)
	if err != nil {
		return err
	}
	err = runScripts(g, manifestDir, "scripts", rootDir, envGetter, buildLog)
	if err != nil {
		return err
	}
	if err := cleanPackages(g, rootDir, buildLog); err != nil {
		return err
	}
	if err := clearResolvConf(g, buildLog, rootDir); err != nil {
		return err
	}
	err = copyFiles(manifestDir, "post-scripts-files", rootDir, buildLog)
	if err != nil {
		return err
	}
	if err := deleteDirectories(directoriesToDelete); err != nil {
		return err
	}
	directoriesToDelete = nil
	err = runScripts(nil, manifestDir, "post-cleanup-scripts", rootDir,
		envGetter, buildLog)
	if err != nil {
		return err
	}
	return nil
}

func copyFiles(manifestDir, dirname, rootDir string, buildLog io.Writer) error {
	startTime := time.Now()
	sourceDir := filepath.Join(manifestDir, dirname)
	cf := func(destFilename, sourceFilename string, mode os.FileMode) error {
		return copyFile(destFilename, sourceFilename, mode, len(manifestDir)+1,
			buildLog)
	}
	if err := fsutil.CopyTreeWithCopyFunc(rootDir, sourceDir, cf); err != nil {
		return fmt.Errorf("error copying %s: %s", dirname, err)
	}
	fmt.Fprintf(buildLog, "\nCopied %s tree in %s\n",
		dirname, format.Duration(time.Since(startTime)))
	return nil
}

func copyFile(destFilename, sourceFilename string, mode os.FileMode,
	prefixLength int, buildLog io.Writer) error {
	same, err := fsutil.CompareFiles(destFilename, sourceFilename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if same {
		fmt.Fprintf(buildLog, "Same contents for: %s\n",
			sourceFilename[prefixLength:])
		return nil
	}
	return fsutil.CopyFile(destFilename, sourceFilename, mode)
}

func installPackages(g *goroutine.Goroutine, packageList []string,
	rootDir string, envGetter environmentGetter, buildLog io.Writer) error {
	if len(packageList) < 1 { // Nothing to do.
		fmt.Fprintln(buildLog, "\nNo packages to install")
		return nil
	}
	fmt.Fprintln(buildLog, "\nUpgrading packages:")
	startTime := time.Now()
	err := runInTarget(g, nil, buildLog, rootDir, envGetter,
		packagerPathname, "upgrade")
	if err != nil {
		return errors.New("error upgrading: " + err.Error())
	}
	fmt.Fprintf(buildLog, "Package upgrade took: %s\n",
		format.Duration(time.Since(startTime)))

	fmt.Fprintln(buildLog, "\nInstalling packages:",
		strings.Join(packageList, " "))
	startTime = time.Now()
	args := []string{"install"}
	args = append(args, packageList...)
	err = runInTarget(g, nil, buildLog, rootDir, envGetter,
		packagerPathname, args...)
	if err != nil {
		return errors.New("error installing: " + err.Error())
	}
	fmt.Fprintf(buildLog, "Package install took: %s\n",
		format.Duration(time.Since(startTime)))
	return nil
}

func runScripts(g *goroutine.Goroutine, manifestDir, dirname, rootDir string,
	envGetter environmentGetter, buildLog io.Writer) error {
	scriptsDir := filepath.Join(manifestDir, dirname)
	file, err := os.Open(scriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(buildLog, "No %s directory\n", dirname)
			return nil
		}
		return err
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return err
	}
	if len(names) < 1 {
		fmt.Fprintln(buildLog, "\nNo scripts to run")
		return nil
	}
	verstr.Sort(names)
	fmt.Fprintf(buildLog, "\nRunning scripts in: %s\n", dirname)
	scriptsStartTime := time.Now()
	tmpDir := filepath.Join(rootDir, ".scripts")
	if err := os.Mkdir(tmpDir, dirPerms); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	for _, name := range names {
		if len(name) > 0 && name[0] == '.' {
			continue // Skip hidden paths.
		}
		err := fsutil.CopyFile(filepath.Join(tmpDir, name),
			filepath.Join(scriptsDir, name),
			dirPerms)
		if err != nil {
			return err
		}
	}
	if g == nil {
		g, err = newNamespaceTargetWithMounts(rootDir, nil)
		if err != nil {
			return err
		}
		defer g.Quit()
	}
	for _, name := range names {
		fmt.Fprintf(buildLog, "Running script: %s\n", name)
		startTime := time.Now()
		err := runInTarget(g, nil, buildLog, rootDir, envGetter,
			packagerPathname, "run", filepath.Join("/.scripts", name))
		if err != nil {
			return errors.New("error running script: " + name + ": " +
				err.Error())
		}
		timeTaken := time.Since(startTime)
		fmt.Fprintf(buildLog, "Script: %s took %s\n",
			name, format.Duration(timeTaken))
		fmt.Fprintln(buildLog,
			"=================================================================")
	}
	timeTaken := time.Since(scriptsStartTime)
	fmt.Fprintf(buildLog, "Ran scripts in %s\n", format.Duration(timeTaken))
	return nil
}

func updatePackageDatabase(g *goroutine.Goroutine, rootDir string,
	envGetter environmentGetter, buildLog io.Writer) error {
	fmt.Fprintln(buildLog, "\nUpdating package database:")
	startTime := time.Now()
	err := runInTarget(g, nil, buildLog, rootDir, envGetter,
		packagerPathname, "update")
	if err != nil {
		return errors.New("error updating: " + err.Error())
	}
	fmt.Fprintf(buildLog, "Package databse update took: %s\n",
		format.Duration(time.Since(startTime)))
	return nil
}
