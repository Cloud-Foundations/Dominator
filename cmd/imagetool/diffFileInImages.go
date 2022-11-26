package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func diffFileInImagesSubcommand(args []string,
	logger log.DebugLogger) error {
	err := diffFileInImages(args[0], args[1], args[2], args[3])
	if err != nil {
		return fmt.Errorf("error diffing files: %s", err)
	}
	return nil
}

func diffFileInImages(tool, leftName, rightName, filename string) error {
	leftReader, err := getTypedFileReader(leftName, filename)
	if err != nil {
		return err
	}
	leftFile, err := copyToTempfile(leftReader)
	leftReader.Close()
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightReader, err := getTypedFileReader(rightName, filename)
	if err != nil {
		return err
	}
	rightFile, err := copyToTempfile(rightReader)
	rightReader.Close()
	if err != nil {
		return err
	}
	defer os.Remove(rightFile)
	cmd := exec.Command(tool, leftFile, rightFile)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func copyToTempfile(reader io.Reader) (string, error) {
	file, err := ioutil.TempFile("", "imagetool-diff")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, reader); err != nil {
		return "", err
	}
	return file.Name(), nil
}
