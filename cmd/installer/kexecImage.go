package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/osutil"
)

func kexecImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := kexecImage(args[0], logger); err != nil {
		return fmt.Errorf("error kexecing image: %s", err)
	}
	return nil
}

func kexecImage(imageName string, logger log.DebugLogger) error {
	_, img, client, err := getImage(imageName, logger)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return err
	}
	fs := img.FileSystem
	fs.FilenameToInodeTable()
	objClient := objectclient.AttachObjectClient(client)
	defer objClient.Close()
	rootDir, err := ioutil.TempDir("", "kexec")
	if err != nil {
		return err
	}
	defer os.RemoveAll(rootDir)
	initrdFilename, err := copyFileInImage(rootDir, fs, objClient, "initrd.img")
	if err != nil {
		return err
	}
	kernelFilename, err := copyFileInImage(rootDir, fs, objClient, "vmlinuz")
	if err != nil {
		return err
	}
	if err := osutil.SyncTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("error syncing: %s\n", err)
	}
	kexec, err := exec.LookPath("kexec")
	if err != nil {
		return err
	}
	var command string
	var args []string
	if os.Geteuid() == 0 {
		command = kexec
	} else {
		command = "sudo"
		args = []string{kexec}
	}
	appendArg := "console=tty0 console=ttyS0,115200n8"
	if *tftpServerHostname != "" {
		appendArg += " tftpserver=" + *tftpServerHostname
	}
	args = append(args, "-l", kernelFilename,
		"--append="+appendArg,
		"--console-serial", "--serial-baud=115200",
		"--console-vga",
		"--initrd="+initrdFilename, "-f")
	if os.Geteuid() == 0 {
		logger.Printf("running kexec with args: %v\n", args)
	} else {
		logger.Printf("running sudo kexec with args: %v\n", args[1:])
	}
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openFileInImage(fs *filesystem.FileSystem,
	objClient *objectclient.ObjectClient,
	filename string) (io.ReadCloser, error) {
	filenameToInodeTable := fs.FilenameToInodeTable()
	if inum, ok := filenameToInodeTable[filename]; !ok {
		return nil, fmt.Errorf("file: \"%s\" not present in image", filename)
	} else if inode, ok := fs.InodeTable[inum]; !ok {
		return nil, fmt.Errorf("inode: %d not present in image", inum)
	} else if inode, ok := inode.(*filesystem.RegularInode); !ok {
		return nil, fmt.Errorf("file: \"%s\" is not a regular file", filename)
	} else {
		_, reader, err := objClient.GetObject(inode.Hash)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}
}

func copyFileInImage(rootDir string, fs *filesystem.FileSystem,
	objClient *objectclient.ObjectClient, filename string) (string, error) {
	sourceFilename := filepath.Join("/", filename)
	reader, err := openFileInImage(fs, objClient, sourceFilename)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	destFilename := filepath.Join(rootDir, filename)
	err = fsutil.CopyToFile(destFilename, fsutil.PublicFilePerms, reader, 0)
	if err != nil {
		return "", err
	}
	return destFilename, nil
}
