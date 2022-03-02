package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func diffImagePackageListsSubcommand(args []string,
	logger log.DebugLogger) error {
	err := diffImagePackageLists(args[0], args[1], args[2])
	if err != nil {
		return fmt.Errorf("error diffing package lists: %s", err)
	}
	return nil
}

func diffImagePackageLists(tool, leftName, rightName string) error {
	leftImage, err := getImageMetadata(leftName)
	if err != nil {
		return err
	}
	if len(leftImage.Packages) < 1 {
		return errors.New("no left image package data")
	}
	rightImage, err := getImageMetadata(rightName)
	if err != nil {
		return err
	}
	if len(rightImage.Packages) < 1 {
		return errors.New("no right image package data")
	}
	var nameWidth, versionWidth int
	for _, pkg := range leftImage.Packages {
		if len(pkg.Name) > nameWidth {
			nameWidth = len(pkg.Name)
		}
		if len(pkg.Version) > versionWidth {
			versionWidth = len(pkg.Version)
		}
	}
	for _, pkg := range rightImage.Packages {
		if len(pkg.Name) > nameWidth {
			nameWidth = len(pkg.Name)
		}
		if len(pkg.Version) > versionWidth {
			versionWidth = len(pkg.Version)
		}
	}
	leftFile, err := writePackageListToTempfile(leftImage.Packages,
		nameWidth, versionWidth)
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightFile, err := writePackageListToTempfile(rightImage.Packages,
		nameWidth, versionWidth)
	if err != nil {
		return err
	}
	defer os.Remove(rightFile)
	cmd := exec.Command(tool, leftFile, rightFile)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func writePackageListToTempfile(packages []image.Package,
	nameWidth, versionWidth int) (string, error) {
	file, err := ioutil.TempFile("", "imagetool-left")
	if err != nil {
		return "", err
	}
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	for _, pkg := range packages {
		fmt.Fprintf(writer, "%-*s %-*s %s\n",
			nameWidth, pkg.Name,
			versionWidth, pkg.Version,
			format.FormatBytes(pkg.Size))
	}
	return file.Name(), nil
}
