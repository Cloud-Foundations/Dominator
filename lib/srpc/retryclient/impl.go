package retryclient

import (
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/retry"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const checkDelay = time.Second

func dialHTTP(params Params) (*RetryClient, error) {
	if params.Dialer == nil {
		params.Dialer = &net.Dialer{}
	}
	client := &RetryClient{params: params}
	if err := client.dial(); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *RetryClient) call(serviceMethod string) (*srpc.Conn, error) {
	if err := client.maybePing(); err != nil {
		return nil, err
	}
	if conn, err := client.client.Call(serviceMethod); err != nil {
		return nil, err
	} else {
		client.lastGoodTime = time.Now()
		return conn, nil
	}
}

func (client *RetryClient) close() error {
	if err := client.client.Close(); err != nil {
		return err
	}
	client.client = nil
	return nil
}

func (client *RetryClient) dial() error {
	return retry.Retry(func() bool {
		if err := client.dialOnce(); err == nil {
			return true
		}
		return false
	}, client.params.Params)
}

func (client *RetryClient) dialOnce() error {
	if client.client != nil {
		client.client.Close()
		client.client = nil
	}
	rawClient, err := srpc.DialTlsHTTPWithDialer(client.params.Network,
		client.params.Address, client.params.TlsConfig, client.params.Dialer)
	if err != nil {
		return err
	}
	client.lastGoodTime = time.Now()
	if client.params.KeepAlive {
		if err := rawClient.SetKeepAlive(client.params.KeepAlive); err != nil {
			rawClient.Close()
			return err
		}
	}
	if client.params.KeepAlivePeriod > 0 {
		err := rawClient.SetKeepAlivePeriod(client.params.KeepAlivePeriod)
		if err != nil {
			rawClient.Close()
			return err
		}
	}
	client.client = rawClient
	return nil
}

func (client *RetryClient) maybePing() error {
	if time.Since(client.lastGoodTime) < checkDelay {
		return nil
	}
	return client.Ping()
}

func (client *RetryClient) ping() error {
	if client.client.Ping() == nil {
		client.lastGoodTime = time.Now()
		return nil
	}
	return client.dial()
}

func (client *RetryClient) requestReply(serviceMethod string,
	request interface{}, reply interface{}) error {
	if err := client.maybePing(); err != nil {
		return err
	}
	err := client.client.RequestReply(serviceMethod, request, reply)
	if err != nil {
		return err
	}
	client.lastGoodTime = time.Now()
	return nil
}

func (client *RetryClient) setKeepAlive(keepAlive bool) error {
	client.params.KeepAlive = keepAlive
	if client == nil {
		return nil
	}
	return client.client.SetKeepAlive(keepAlive)
}

func (client *RetryClient) setKeepAlivePeriod(d time.Duration) error {
	client.params.KeepAlivePeriod = d
	if client == nil {
		return nil
	}
	return client.client.SetKeepAlivePeriod(d)
}
