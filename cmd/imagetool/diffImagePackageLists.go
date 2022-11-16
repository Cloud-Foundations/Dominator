package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

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
	leftPackages, err := getTypedPackageList(leftName)
	if err != nil {
		return err
	}
	if len(leftPackages) < 1 {
		return errors.New("no left image package data")
	}
	rightPackages, err := getTypedPackageList(rightName)
	if err != nil {
		return err
	}
	if len(rightPackages) < 1 {
		return errors.New("no right image package data")
	}
	var nameWidth, versionWidth int
	getWidthsForPackages(leftPackages, &nameWidth, &versionWidth)
	getWidthsForPackages(rightPackages, &nameWidth, &versionWidth)
	leftFile, err := writePackageListToTempfile(leftPackages,
		nameWidth, versionWidth)
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightFile, err := writePackageListToTempfile(rightPackages,
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
	file, err := ioutil.TempFile("", "imagetool-diff")
	if err != nil {
		return "", err
	}
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	listPackages(writer, packages, nameWidth, versionWidth)
	return file.Name(), nil
}
