package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
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
		if lFilter != nil && rFilter == nil {
			filt = lFilter
		} else if lFilter == nil && rFilter != nil {
			filt = rFilter
		} else if lFilter.Equal(rFilter) {
			filt = lFilter
		}
		if lfs, err = applyDeleteFilter(lfs, filt); err != nil {
			return fmt.Errorf("error filtering left image: %s", err)
		}
		if rfs, err = applyDeleteFilter(rfs, filt); err != nil {
			return fmt.Errorf("error filtering right image: %s", err)
		}
	}
	err = diffImages(tool, lfs, rfs)
	if err != nil {
		return fmt.Errorf("error diffing images: %s", err)
	}
	return nil
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
