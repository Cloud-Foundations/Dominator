package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getImagePackageListSubcommand(args []string,
	logger log.DebugLogger) error {
	var outFileName string
	if len(args) > 1 {
		outFileName = args[1]
	}
	err := getImagePackageList(args[0], outFileName)
	if err != nil {
		return fmt.Errorf("error getting image package list: %s", err)
	}
	return nil
}

func getImagePackageList(imageName, outFileName string) error {
	img, err := getImageMetadata(imageName)
	if err != nil {
		return err
	}
	if len(img.Packages) < 1 {
		return errors.New("no package data")
	}
	var writer io.Writer
	if outFileName == "" {
		writer = os.Stdout
	} else {
		w, err := os.OpenFile(outFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			fsutil.PublicFilePerms)
		if err != nil {
			return err
		}
		defer w.Close()
		bw := bufio.NewWriter(w)
		defer bw.Flush()
		writer = bw
	}
	var nameWidth, versionWidth int
	getWidthsForPackages(img.Packages, &nameWidth, &versionWidth)
	listPackages(writer, img.Packages, nameWidth, versionWidth)
	return nil
}

func getWidthsForPackages(packages []image.Package,
	nameWidth, versionWidth *int) {
	for _, pkg := range packages {
		if len(pkg.Name) > *nameWidth {
			*nameWidth = len(pkg.Name)
		}
		if len(pkg.Version) > *versionWidth {
			*versionWidth = len(pkg.Version)
		}
	}
}

func listPackages(writer io.Writer, packages []image.Package,
	nameWidth, versionWidth int) {
	for _, pkg := range packages {
		fmt.Fprintf(writer, "%-*s %-*s %s\n",
			nameWidth, pkg.Name,
			versionWidth, pkg.Version,
			format.FormatBytes(pkg.Size))
	}
}
