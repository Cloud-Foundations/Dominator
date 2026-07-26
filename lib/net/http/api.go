package http

import (
	"io"
	"net/http"
)

type Client interface {
	// GetData returns data from the specified URL, limiting concurrency until
	// the data are fetched.
	GetData(url string) ([]byte, error)

	// GetReader returns a ReadCloser from the specified URL. Concurrency is
	// limited until the Close() method is called.
	GetReader(url string) (io.ReadCloser, uint64, error)
}

// NewLimitedConcurrencyClient creates a simplified HTTP client which limits
// concurrency during I/O operations. The number of concurrent operations is
// limited by the size of ioSemaphore.
func NewLimitedConcurrencyClient(ioSemaphore chan struct{}) Client {
	return newLimitedConcurrencyClient(ioSemaphore)
}

// GetData returns data from the specified URL using the specified client.
// If httpClient is nil the default client is used.
func GetData(httpClient *http.Client, url string) ([]byte, error) {
	return getData(httpClient, url)
}

// GetReader returns a ReadCloser from the specified URL using the specified
// client.
// If httpClient is nil the default client is used.
func GetReader(httpClient *http.Client, url string) (
	io.ReadCloser, uint64, error) {
	return getReader(httpClient, url)
}
