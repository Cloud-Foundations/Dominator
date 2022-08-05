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

func watchUrl(rawurl string, checkInterval time.Duration,
	logger log.DebugLogger) (<-chan io.ReadCloser, error) {
	if strings.HasPrefix(rawurl, "git@") {
		pos := strings.Index(rawurl, ".git/")
		if pos < 5 {
			return nil, errors.New("missing .git/ in Git URL")
		}
		if pos+5 >= len(rawurl) {
			return nil, errors.New("missing path in repository")
		}
		ch := make(chan io.ReadCloser, 1)
		go watchGitLoop(rawurl[:pos+4], rawurl[pos+5:], checkInterval, ch,
			logger)
		return ch, nil
	}
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "file" {
		return fsutil.WatchFile(u.Path, logger), nil
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		ch := make(chan io.ReadCloser, 1)
		go watchUrlLoop(rawurl, checkInterval, ch, logger)
		return ch, nil
	}
	return nil, errors.New("unknown scheme: " + u.Scheme)
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

func watchUrlLoop(rawurl string, checkInterval time.Duration,
	ch chan<- io.ReadCloser, logger log.Logger) {
	for ; ; time.Sleep(checkInterval) {
		watchUrlOnce(rawurl, ch, logger)
		if checkInterval <= 0 {
			return
		}
	}
}

func watchUrlOnce(rawurl string, ch chan<- io.ReadCloser, logger log.Logger) {
	resp, err := http.Get(rawurl)
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
