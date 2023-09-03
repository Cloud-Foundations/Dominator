// +build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/lib/decoders"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

const filePerms = syscall.S_IRUSR | syscall.S_IRGRP | syscall.S_IROTH

type logWriterType struct {
	buffer bytes.Buffer
}

func buildFromManifestSubcommand(args []string, logger log.DebugLogger) error {
	srpcClient := getImageServerClient()
	logWriter := &logWriterType{}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "Start of build log ==========================")
	}
	var variables map[string]string
	if *variablesFilename != "" {
		err := decoders.DecodeFile(*variablesFilename, &variables)
		if err != nil {
			return err
		}
	}
	name, err := builder.BuildImageFromManifestWithOptions(
		srpcClient,
		builder.BuildLocalOptions{
			BindMounts:        bindMounts,
			ManifestDirectory: args[0],
			Variables:         variables,
		},
		args[1],
		*expiresIn,
		logWriter)
	if err != nil {
		if !*alwaysShowBuildLog {
			fmt.Fprintln(os.Stderr,
				"Start of build log ==========================")
			os.Stderr.Write(logWriter.Bytes())
		}
		fmt.Fprintln(os.Stderr, "End of build log ============================")
		return fmt.Errorf("error processing manifest: %s", err)
	}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "End of build log ============================")
	} else {
		err := fsutil.CopyToFile("build.log", filePerms, &logWriter.buffer,
			uint64(logWriter.buffer.Len()))
		if err != nil {
			return fmt.Errorf("error writing build log: %s", err)
		}
	}
	fmt.Println(name)
	return nil
}

func buildTreeFromManifestSubcommand(args []string,
	logger log.DebugLogger) error {
	srpcClient := getImageServerClient()
	logWriter := &logWriterType{}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "Start of build log ==========================")
	}
	var variables map[string]string
	if *variablesFilename != "" {
		err := decoders.DecodeFile(*variablesFilename, &variables)
		if err != nil {
			return err
		}
	}
	rootDir, err := builder.BuildTreeFromManifestWithOptions(
		srpcClient,
		builder.BuildLocalOptions{
			BindMounts:        bindMounts,
			ManifestDirectory: args[0],
			Variables:         variables,
		},
		logWriter)
	if err != nil {
		if !*alwaysShowBuildLog {
			fmt.Fprintln(os.Stderr,
				"Start of build log ==========================")
			os.Stderr.Write(logWriter.Bytes())
		}
		fmt.Fprintln(os.Stderr, "End of build log ============================")
		return fmt.Errorf("error processing manifest: %s", err)
	}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "End of build log ============================")
	} else {
		err := fsutil.CopyToFile("build.log", filePerms, &logWriter.buffer,
			uint64(logWriter.buffer.Len()))
		if err != nil {
			return fmt.Errorf("error writing build log: %s", err)
		}
	}
	fmt.Println(rootDir)
	return nil
}

func processManifestSubcommand(args []string, logger log.DebugLogger) error {
	logWriter := &logWriterType{}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "Start of build log ==========================")
	}
	var variables map[string]string
	if *variablesFilename != "" {
		err := decoders.DecodeFile(*variablesFilename, &variables)
		if err != nil {
			return err
		}
	}
	err := builder.ProcessManifestWithOptions(
		builder.BuildLocalOptions{
			BindMounts:        bindMounts,
			ManifestDirectory: args[0],
			Variables:         variables,
		},
		args[1], logWriter)
	if err != nil {
		if !*alwaysShowBuildLog {
			fmt.Fprintln(os.Stderr,
				"Start of build log ==========================")
			os.Stderr.Write(logWriter.Bytes())
		}
		fmt.Fprintln(os.Stderr, "End of build log ============================")
		return fmt.Errorf("error processing manifest: %s", err)
	}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "End of build log ============================")
	} else {
		err := fsutil.CopyToFile("build.log", filePerms, &logWriter.buffer,
			uint64(logWriter.buffer.Len()))
		if err != nil {
			return fmt.Errorf("error writing build log: %s", err)
		}
	}
	return nil
}

func (w *logWriterType) Bytes() []byte {
	return w.buffer.Bytes()
}

func (w *logWriterType) Write(p []byte) (int, error) {
	if *alwaysShowBuildLog {
		os.Stderr.Write(p)
	}
	return w.buffer.Write(p)
}
