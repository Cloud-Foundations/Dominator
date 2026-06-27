package urlutil

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
)

type sizedReadCloser struct {
	io.ReadCloser
	size uint64
}

func open(rawurl string) (*sizedReadCloser, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "file" {
		return openFile(u.Path)
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return openHttp(rawurl)
	}
	return nil, errors.New("unknown scheme: " + u.Scheme)
}

func openFile(filename string) (*sizedReadCloser, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	doClose := true
	defer func() {
		if doClose {
			file.Close()
		}
	}()
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	doClose = false
	return &sizedReadCloser{
		ReadCloser: file,
		size:       uint64(fi.Size()),
	}, nil
}

func openHttp(rawurl string) (*sizedReadCloser, error) {
	resp, err := http.Get(rawurl)
	if err != nil {
		return nil, errors.New("error getting: " + rawurl + ": " + err.Error())
	}
	doClose := true
	defer func() {
		if doClose {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("error getting: " + rawurl + ": " + resp.Status)
	}
	if resp.ContentLength < 0 {
		return nil, errors.New("ContentLength from: " + rawurl)
	}
	doClose = false
	return &sizedReadCloser{
		ReadCloser: resp.Body,
		size:       uint64(resp.ContentLength),
	}, nil
}

func (src *sizedReadCloser) Size() uint64 {
	return src.size
}
