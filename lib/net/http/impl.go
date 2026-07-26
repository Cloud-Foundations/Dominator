package http

import (
	"fmt"
	"io"
	"net/http"
)

type clientType struct {
	httpClient  *http.Client
	ioSemaphore chan struct{}
}

type readCloser struct {
	ioSemaphore <-chan struct{}
	rc          io.ReadCloser
}

func getData(httpClient *http.Client, url string) ([]byte, error) {
	rc, _, err := GetReader(httpClient, url)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func getReader(httpClient *http.Client, url string) (
	io.ReadCloser, uint64, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("Get \"%s\": %s", url, resp.Status)
	}
	if resp.ContentLength < 0 {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("Get \"%s\": unknown content length", url)
	}
	return resp.Body, uint64(resp.ContentLength), nil
}

func newLimitedConcurrencyClient(ioSemaphore chan struct{}) Client {
	return &clientType{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxConnsPerHost: cap(ioSemaphore),
			},
		},
		ioSemaphore: ioSemaphore,
	}
}

func (c *clientType) GetData(url string) ([]byte, error) {
	rc, _, err := c.GetReader(url)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (c *clientType) GetReader(url string) (io.ReadCloser, uint64, error) {
	c.ioSemaphore <- struct{}{}
	rc, size, err := GetReader(c.httpClient, url)
	if err != nil {
		<-c.ioSemaphore
		return nil, 0, err
	}
	return &readCloser{
		ioSemaphore: c.ioSemaphore,
		rc:          rc,
	}, size, nil
}

func (rc *readCloser) Close() error {
	err := rc.rc.Close()
	<-rc.ioSemaphore
	return err
}

func (rc *readCloser) Read(p []byte) (int, error) {
	return rc.rc.Read(p)
}
