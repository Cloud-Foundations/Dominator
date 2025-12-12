package retryclient

import (
	"crypto/tls"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/retry"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var (
	// Interface check.
	_ srpc.ClientI = (*RetryClient)(nil)
)

type Params struct {
	retry.Params
	Address         string
	Dialer          srpc.Dialer
	KeepAlive       bool
	KeepAlivePeriod time.Duration
	Network         string
	TlsConfig       *tls.Config
}

type RetryClient struct {
	params       Params
	client       *srpc.Client
	closeError   error
	lastGoodTime time.Time
}

func DialHTTP(params Params) (*RetryClient, error) {
	return dialHTTP(params)
}

func (client *RetryClient) Call(serviceMethod string) (*srpc.Conn, error) {
	return client.call(serviceMethod)
}

func (client *RetryClient) Close() error {
	return client.close()
}

func (client *RetryClient) Ping() error {
	return client.ping()
}

func (client *RetryClient) RequestReply(serviceMethod string,
	request interface{}, reply interface{}) error {
	return client.requestReply(serviceMethod, request, reply)
}

func (client *RetryClient) SetKeepAlive(keepAlive bool) error {
	return client.setKeepAlive(keepAlive)
}

func (client *RetryClient) SetKeepAlivePeriod(d time.Duration) error {
	return client.setKeepAlivePeriod(d)
}
