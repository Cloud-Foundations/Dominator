package builder

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/expand"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

const (
	cmdPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
	packagerPathname = "/bin/generic-packager"
)

var environmentToCopy = map[string]struct{}{
	"PATH":  {},
	"TZ":    {},
	"SHELL": {},
}

var environmentToSet = map[string]string{
	"HOME":    "/",
	"LOGNAME": "root",
	"USER":    "root",
}

func cleanPackages(ctx context.Context, g *goroutine.Goroutine, rootDir string,
	buildLog io.Writer) error {
	fmt.Fprintln(buildLog, "\nCleaning packages:")
	startTime := time.Now()
	err := runInTarget(ctx, g, nil, buildLog, buildLog, rootDir, nil,
		packagerPathname, "clean")
	if err != nil {
		return errors.New("error cleaning: " + err.Error())
	}
	stdout := &bytes.Buffer{}
	err = runInTarget(ctx, g, nil, stdout, buildLog, rootDir, nil,
		packagerPathname, "show-clean-patterns")
	if err != nil {
		fmt.Fprintf(buildLog, "Deep clean failed: %s\n", err)
	} else {
		filter, err := filter.Read(stdout)
		if err != nil {
			return err
		}
		if err := util.DeleteFilteredFiles(rootDir, filter); err != nil {
			return err
		}
	}
	fmt.Fprintf(buildLog, "Package clean took: %s\n",
		format.Duration(time.Since(startTime)))
	return nil
}

func clearResolvConf(ctx context.Context, g *goroutine.Goroutine,
	writer io.Writer, rootDir string) error {
	return runInTarget(ctx, g, nil, writer, writer, rootDir,
		nil,
		"/bin/cp", "/dev/null", "/etc/resolv.conf")
}

func makeTempDirectory(dir, prefix string) (string, error) {
	tmpDir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(tmpDir, dirPerms); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpDir, nil
}

func (stream *bootstrapStream) build(b *Builder, client srpc.ClientI,
	request proto.BuildImageRequest,
	buildLog buildLogger) (*image.Image, error) {
	startTime := time.Now()
	args := make([]string, 0, len(stream.BootstrapCommand))
	rootDir, err := makeTempDirectory("",
		strings.Replace(request.StreamName, "/", "_", -1))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(rootDir)
	fmt.Fprintf(buildLog, "Created image working directory: %s\n", rootDir)
	vg := variablesGetter(request.Variables).copy()
	vg.add("dir", rootDir)
	request.Variables = vg
	for _, exp := range stream.BootstrapCommand {
		arg := expand.Expression(exp, func(name string) string {
			return vg[name]
		})
		args = append(args, arg)
	}
	fmt.Fprintf(buildLog, "Running command: %s with args:\n", args[0])
	for _, arg := range args[1:] {
		fmt.Fprintf(buildLog, "    %s\n", arg)
	}
	g, err := newNamespaceTarget()
	if err != nil {
		return nil, err
	}
	defer g.Quit()
	ctx, cancel := makeContext(b.maximumBuildDuration)
	defer cancel()
	err = runInTarget(ctx, g, nil, buildLog, buildLog, "", nil,
		args[0], args[1:]...)
	if err != nil {
		return nil, err
	} else {
		packager := b.packagerTypes[stream.PackagerType]
		if err := packager.writePackageInstaller(rootDir); err != nil {
			return nil, err
		}
		if err := clearResolvConf(ctx, g, buildLog, rootDir); err != nil {
			return nil, err
		}
		buildDuration := time.Since(startTime)
		fmt.Fprintf(buildLog, "\nBuild time: %s\n",
			format.Duration(buildDuration))
		if err := cleanPackages(ctx, g, rootDir, buildLog); err != nil {
			return nil, err
		}
		return packImage(ctx, g, client, request, rootDir,
			stream.Filter, nil, nil, stream.imageFilter, stream.imageTags,
			stream.imageTriggers, b.mtimesCopyFilter, buildLog, b.logger)
	}
}

func (packager *packagerType) writePackageInstaller(rootDir string) error {
	filename := filepath.Join(rootDir, packagerPathname)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, cmdPerms)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	packager.writePackageInstallerContents(writer)
	return writer.Flush()
}

func (packager *packagerType) writePackageInstallerContents(writer io.Writer) {
	fmt.Fprintln(writer, "#! /bin/sh")
	fmt.Fprintln(writer, "# Created by imaginator.")
	for _, line := range packager.Verbatim {
		fmt.Fprintln(writer, line)
	}
	fmt.Fprintln(writer, "cmd=\"$1\"; shift")
	writePackagerCommand(writer, "clean", packager.CleanCommand)
	fmt.Fprintln(writer, `[ "$cmd" = "copy-in" ] && exec cat > "$1"`)
	writePackagerCommand(writer, "install", packager.InstallCommand)
	writePackagerCommand(writer, "list", packager.ListCommand.ArgList)
	writePackagerCommand(writer, "remove", packager.RemoveCommand)
	fmt.Fprintln(writer, `[ "$cmd" = "run" ] && exec "$@"`)
	multiplier := packager.ListCommand.SizeMultiplier
	if multiplier < 1 {
		multiplier = 1
	}
	fmt.Fprintln(writer, `if [ "$cmd" = "show-clean-patterns" ]; then`)
	for _, line := range packager.CleanPatterns {
		fmt.Fprintf(writer, "    echo '%s'\n", line)
	}
	fmt.Fprintln(writer, `    exit 0`)
	fmt.Fprintln(writer, `fi`)
	fmt.Fprintf(writer,
		"[ \"$cmd\" = \"show-size-multiplier\" ] && exec echo %d\n", multiplier)
	writePackagerCommand(writer, "update", packager.UpdateCommand)
	writePackagerCommand(writer, "upgrade", packager.UpgradeCommand)
	fmt.Fprintln(writer, "echo \"Invalid command: $cmd\"")
	fmt.Fprintln(writer, "exit 2")
}

func writePackagerCommand(writer io.Writer, cmd string, command []string) {
	if len(command) < 1 {
		fmt.Fprintf(writer, "[ \"$cmd\" = \"%s\" ] && exit 0\n", cmd)
	} else {
		fmt.Fprintf(writer, "[ \"$cmd\" = \"%s\" ] && exec", cmd)
		for _, arg := range command {
			writeArgument(writer, arg)
		}
		fmt.Fprintf(writer, " \"$@\"\n")
	}
}

func writeArgument(writer io.Writer, arg string) {
	if len(strings.Fields(arg)) < 2 {
		fmt.Fprintf(writer, " %s", arg)
	} else {
		lenArg := len(arg)
		if lenArg > 0 && arg[lenArg-1] == '\n' {
			arg = arg[:lenArg-1] + `\n`
		}
		fmt.Fprintf(writer, " '%s'", arg)
	}
}
