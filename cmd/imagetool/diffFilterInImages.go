package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func diffFilterInImagesSubcommand(args []string,
	logger log.DebugLogger) error {
	err := diffFilterInImages(args[0], args[1], args[2])
	if err != nil {
		return fmt.Errorf("error diffing files: %s", err)
	}
	return nil
}

func diffFilterInImages(tool, leftName, rightName string) error {
	leftFilter, err := getTypedImageFilter(leftName)
	if err != nil {
		return err
	}
	rightFilter, err := getTypedImageFilter(rightName)
	if err != nil {
		return err
	}
	leftFile, err := writeToTempfile(leftFilter.Write)
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightFile, err := writeToTempfile(rightFilter.Write)
	if err != nil {
		return err
	}
	defer os.Remove(rightFile)
	cmd := exec.Command(tool, leftFile, rightFile)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func writeToTempfile(writerFunc func(io.Writer) error) (string, error) {
	file, err := ioutil.TempFile("", "imagetool-diff")
	if err != nil {
		return "", err
	}
	doCleanup := true
	defer func() {
		if doCleanup {
			os.Remove(file.Name())
			file.Close()
		}
	}()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	if err := writerFunc(writer); err != nil {
		return "", err
	}
	doCleanup = false
	return file.Name(), nil
}
