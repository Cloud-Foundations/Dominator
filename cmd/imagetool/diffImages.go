package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func diffSubcommand(args []string, logger log.DebugLogger) error {
	return diffTypedImages(args[0], args[1], args[2])
}

func diffTypedImages(tool string, lName string, rName string) error {
	lfs, lFilter, err := getTypedFileSystemAndFilter(lName)
	if err != nil {
		return fmt.Errorf("error getting left image: %s", err)
	}
	rfs, rFilter, err := getTypedFileSystemAndFilter(rName)
	if err != nil {
		return fmt.Errorf("error getting right image: %s", err)
	}
	if !*ignoreFilters {
		var filt *filter.Filter
		// If only one filter is available, use it (common case of comparing an
		// image and a sub. When comparing images (with filters), take the easy
		// way out and compare the full images. If there are no filters and one
		// image is much smaller than another, assume we're comparing a sparse
		// image with a sub and just compare the files in the smaller image with
		// the larger image. This makes the diff reasonable.
		if lFilter != nil && rFilter == nil {
			filt = lFilter
		} else if lFilter == nil && rFilter != nil {
			filt = rFilter
		} else if lFilter == nil && rFilter == nil {
			if lfs.NumRegularInodes>>4 > rfs.NumRegularInodes {
				lfs = lfs.FilterUsingReference(rfs)
			} else if rfs.NumRegularInodes>>4 > lfs.NumRegularInodes {
				rfs = rfs.FilterUsingReference(lfs)
			}
		} else if lFilter.Equal(rFilter) {
			filt = lFilter
		}
		if filt != nil {
			startTime := time.Now()
			if err := filt.Compile(); err != nil {
				return err
			}
			var leftError error
			rightErrorChannel := make(chan error, 1)
			go applyDeleteFilterBackground(&rfs, filt, rightErrorChannel)
			lfs, leftError = applyDeleteFilter(lfs, filt)
			rightError := <-rightErrorChannel
			if leftError != nil {
				return fmt.Errorf("error filtering left image: %s", leftError)
			}
			if rightError != nil {
				return fmt.Errorf("error filtering right image: %s", rightError)
			}
			logger.Debugf(0, "applied filter in %s\n",
				format.Duration(time.Since(startTime)))
		}
	}
	err = diffImages(tool, lfs, rfs)
	if err != nil {
		return fmt.Errorf("error diffing images: %s", err)
	}
	return nil
}

func applyDeleteFilterBackground(fs **filesystem.FileSystem,
	filt *filter.Filter, errorChannel chan<- error) {
	newFs, err := applyDeleteFilter(*fs, filt)
	*fs = newFs
	errorChannel <- err
}

func diffImages(tool string, lfs, rfs *filesystem.FileSystem) error {
	lname, err := writeImage(lfs)
	defer os.Remove(lname)
	if err != nil {
		return err
	}
	rname, err := writeImage(rfs)
	defer os.Remove(rname)
	if err != nil {
		return err
	}
	cmd := exec.Command(tool, lname, rname)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func writeImage(fs *filesystem.FileSystem) (string, error) {
	file, err := ioutil.TempFile("", "imagetool")
	if err != nil {
		return "", err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	return file.Name(), fs.Listf(writer, listSelector, listFilter)
}
