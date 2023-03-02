package logarchiver

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

type buildLogArchiver struct {
	options BuildLogArchiveOptions
	params  BuildLogArchiveParams
}

func newBuildLogArchive(options BuildLogArchiveOptions,
	params BuildLogArchiveParams) (*buildLogArchiver, error) {
	archive := &buildLogArchiver{options, params}
	return archive, nil
}

// writeString will write the specified string data and a trailing newline to
// the file specified by filename. If the string is empty, the file is not
// written.
func writeString(filename, data string) error {
	if data == "" {
		return nil
	}
	return ioutil.WriteFile(filename, []byte(data+"\n"), fsutil.PublicFilePerms)
}

func (a *buildLogArchiver) AddBuildLog(buildInfo BuildInfo,
	buildLog []byte) error {
	dirname := filepath.Join(a.options.Topdir, buildInfo.ImageName)
	if err := os.MkdirAll(filepath.Dir(dirname), fsutil.DirPerms); err != nil {
		return err
	}
	if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
		return err
	}
	doDelete := true
	defer func() {
		if doDelete {
			os.RemoveAll(dirname)
		}
	}()
	err := writeString(filepath.Join(dirname, "error"),
		errors.ErrorToString(buildInfo.Error))
	if err != nil {
		return err
	}
	logfile := filepath.Join(dirname, "buildLog")
	err = ioutil.WriteFile(logfile, buildLog, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	err = writeString(filepath.Join(dirname, "requestorUsername"),
		buildInfo.RequestorUsername)
	if err != nil {
		return err
	}
	doDelete = false
	return nil
}
