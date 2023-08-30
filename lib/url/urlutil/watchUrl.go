package urlutil

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/gitutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

const (
	driverFile = iota
	driverGit
	driverHttp
)

type driverDataType struct {
	driverType uint
	gitUrl     string
	pathname   string
	rawUrl     string
}

func parseUrl(rawUrl string) (*driverDataType, error) {
	if strings.HasPrefix(rawUrl, "git@") {
		pos := strings.Index(rawUrl, ".git/")
		if pos < 5 {
			return nil, errors.New("missing .git/ in Git URL")
		}
		if pos+5 >= len(rawUrl) {
			return nil, errors.New("missing path in repository")
		}
		return &driverDataType{
			driverType: driverGit,
			gitUrl:     rawUrl[:pos+4],
			pathname:   rawUrl[pos+5:],
		}, nil
	}
	if rawUrl[0] == '/' {
		return &driverDataType{
			driverType: driverFile,
			pathname:   rawUrl,
		}, nil
	}
	u, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "file" {
		return &driverDataType{
			driverType: driverFile,
			pathname:   u.Path,
		}, nil
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		if strings.HasSuffix(u.Path, ".git") {
			if len(u.RawQuery) < 1 {
				return nil, errors.New("missing path in repository")
			}
			return &driverDataType{
				driverType: driverGit,
				gitUrl:     u.Scheme + "://" + u.Host + u.Path,
				pathname:   u.RawQuery,
			}, nil
		}
		return &driverDataType{
			driverType: driverHttp,
			rawUrl:     rawUrl,
		}, nil
	}
	return nil, errors.New("unknown scheme: " + u.Scheme)
}

func watchUrl(rawUrl string, checkInterval time.Duration,
	logger log.DebugLogger) (<-chan io.ReadCloser, error) {
	driverData, err := parseUrl(rawUrl)
	if err != nil {
		return nil, err
	}
	switch driverData.driverType {
	case driverFile:
		return fsutil.WatchFile(driverData.pathname, logger), nil
	case driverGit:
		ch := make(chan io.ReadCloser, 1)
		go watchGitLoop(driverData.gitUrl, driverData.pathname, checkInterval,
			ch, logger)
		return ch, nil
	case driverHttp:
		ch := make(chan io.ReadCloser, 1)
		go watchUrlLoop(driverData.rawUrl, checkInterval, ch, logger)
		return ch, nil
	}
	return nil, errors.New("unknown driver")
}

func watchGitLoop(gitUrl, pathInRepo string, checkInterval time.Duration,
	ch chan<- io.ReadCloser, logger log.DebugLogger) {
	for ; ; time.Sleep(checkInterval) {
		watchGitOnce(gitUrl, pathInRepo, ch, logger)
		if checkInterval <= 0 {
			return
		}
	}
}

func watchGitOnce(gitUrl, pathInRepo string, ch chan<- io.ReadCloser,
	logger log.DebugLogger) {
	topdir, err := ioutil.TempDir("", "watchGitOnce")
	if err != nil {
		logger.Println(err)
		return
	}
	defer os.RemoveAll(topdir)
	err = gitutil.ShallowClone(topdir, gitutil.ShallowCloneParams{
		Patterns: []string{pathInRepo},
		RepoURL:  gitUrl,
	}, logger)
	if err != nil {
		logger.Println(err)
		return
	}
	filename := filepath.Join(topdir, pathInRepo)
	if file, err := os.Open(filename); err != nil {
		logger.Println(err)
		return
	} else {
		ch <- file
	}
}

func watchUrlLoop(rawUrl string, checkInterval time.Duration,
	ch chan<- io.ReadCloser, logger log.Logger) {
	for ; ; time.Sleep(checkInterval) {
		watchUrlOnce(rawUrl, ch, logger)
		if checkInterval <= 0 {
			return
		}
	}
}

func watchUrlOnce(rawUrl string, ch chan<- io.ReadCloser, logger log.Logger) {
	resp, err := http.Get(rawUrl)
	if err != nil {
		logger.Println(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logger.Println(resp.Status)
		return
	}
	ch <- resp.Body
}
