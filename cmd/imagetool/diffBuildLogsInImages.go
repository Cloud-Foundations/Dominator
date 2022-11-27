package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type printableReader struct {
	reader io.Reader
}

func diffBuildLogsInImagesSubcommand(args []string,
	logger log.DebugLogger) error {
	err := diffBuildLogsInImages(args[0], args[1], args[2])
	if err != nil {
		return fmt.Errorf("error diffing build logs: %s", err)
	}
	return nil
}

func diffBuildLogsInImages(tool, leftName, rightName string) error {
	leftReader, err := getTypedImageBuildLogReader(leftName)
	if err != nil {
		return err
	}
	leftFile, err := copyToTempfile(&printableReader{leftReader})
	leftReader.Close()
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightReader, err := getTypedImageBuildLogReader(rightName)
	if err != nil {
		return err
	}
	rightFile, err := copyToTempfile(&printableReader{rightReader})
	rightReader.Close()
	if err != nil {
		return err
	}
	defer os.Remove(rightFile)
	cmd := exec.Command(tool, leftFile, rightFile)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Read will read and replace bytes which the tkdiff tool doesn't like.
func (pr *printableReader) Read(p []byte) (int, error) {
	nBytes, err := pr.reader.Read(p)
	for index := 0; index < nBytes; index++ {
		switch p[index] {
		case '':
			p[index] = '\n'
		case 0xc2:
			if index+1 < nBytes && p[index+1] == 0xb5 {
				p[index] = ' '
				p[index+1] = 'u'
			}
		case 0xe2:
			if index+2 < nBytes && p[index+1] == 0x86 && p[index+2] == 0x92 {
				p[index] = ' '
				p[index+1] = '-'
				p[index+2] = '>'
			}
		}
	}
	return nBytes, err
}
