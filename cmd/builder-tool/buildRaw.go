// +build linux

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/lib/decoders"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

const createFlags = os.O_CREATE | os.O_TRUNC | os.O_RDWR

type dummyHasher struct{}

func buildRawFromManifestSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := buildRawFromManifest(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error building RAW image from manifest: %s", err)
	}
	return nil
}

func buildRawFromManifest(manifestDir, rawFilename string,
	logger log.DebugLogger) error {
	var variables map[string]string
	if *variablesFilename != "" {
		err := decoders.DecodeFile(*variablesFilename, &variables)
		if err != nil {
			return err
		}
	}
	if rawSize < 1<<20 {
		return fmt.Errorf("rawSize: %d too small", rawSize)
	}
	err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "")
	if err != nil {
		return fmt.Errorf("error making mounts private: %s", err)
	}
	srpcClient := getImageServerClient()
	tmpFilename := rawFilename + "~"
	file, err := os.OpenFile(tmpFilename, createFlags, fsutil.PrivateFilePerms)
	if err != nil {
		return err
	}
	file.Close()
	defer os.Remove(tmpFilename)
	if err := os.Truncate(tmpFilename, int64(rawSize)); err != nil {
		return err
	}
	if err := mbr.WriteDefault(tmpFilename, mbr.TABLE_TYPE_MSDOS); err != nil {
		return err
	}
	partition := "p1"
	loopDevice, err := fsutil.LoopbackSetupAndWaitForPartition(tmpFilename,
		partition, time.Minute, logger)
	if err != nil {
		return err
	}
	defer fsutil.LoopbackDelete(loopDevice)
	rootDevice := loopDevice + partition
	rootLabel := "root@test"
	err = util.MakeExt4fs(rootDevice, rootLabel, nil, 0, logger)
	if err != nil {
		return err
	}
	rootDir, err := ioutil.TempDir("", "rootfs")
	if err != nil {
		return err
	}
	defer os.Remove(rootDir)
	err = wsyscall.Mount(rootDevice, rootDir, "ext4", 0, "")
	if err != nil {
		return fmt.Errorf("error mounting: %s", rootDevice)
	}
	defer syscall.Unmount(rootDir, 0)
	logWriter := &logWriterType{}
	if *alwaysShowBuildLog {
		fmt.Fprintln(os.Stderr, "Start of build log ==========================")
	}
	err = builder.UnpackImageAndProcessManifestWithOptions(
		srpcClient,
		builder.BuildLocalOptions{
			BindMounts:        bindMounts,
			ManifestDirectory: manifestDir,
			Variables:         variables,
		},
		rootDir,
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
	fs, err := scanner.ScanFileSystem(rootDir, nil, nil, nil, &dummyHasher{},
		nil)
	err = util.MakeBootable(&fs.FileSystem, loopDevice, rootLabel, rootDir,
		"net.ifnames=0", false, logger)
	if err != nil {
		return err
	}
	return os.Rename(tmpFilename, rawFilename)
}

func (h *dummyHasher) Hash(reader io.Reader, length uint64) (hash.Hash, error) {
	return hash.Hash{}, nil
}
