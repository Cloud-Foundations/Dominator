package grpc

import (
	"context"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamingConn wraps a gRPC stream to implement srpc.StreamingConn interface.
type StreamingConn struct {
	ctx        context.Context
	authInfo   *srpc.AuthInformation
	decodeFunc func(v any) error
	encodeFunc func(v any) error
	readFunc   func(p []byte) (int, error)
	peekFunc   func(n int) ([]byte, error)
}

type StreamingConnOption func(*StreamingConn)

func WithReadFunc(readFunc func(p []byte) (int, error)) StreamingConnOption {
	return func(c *StreamingConn) {
		c.readFunc = readFunc
	}
}

func WithPeekFunc(peekFunc func(n int) ([]byte, error)) StreamingConnOption {
	return func(c *StreamingConn) {
		c.peekFunc = peekFunc
	}
}

func NewStreamingConn(
	ctx context.Context,
	decodeFunc func(v any) error,
	encodeFunc func(v any) error,
	opts ...StreamingConnOption,
) *StreamingConn {
	conn := ConnFromContext(ctx)
	streamConn := &StreamingConn{
		ctx:        ctx,
		authInfo:   conn.GetAuthInformation(),
		decodeFunc: decodeFunc,
		encodeFunc: encodeFunc,
	}

	// Apply options
	for _, opt := range opts {
		opt(streamConn)
	}

	return streamConn
}

func (c *StreamingConn) Decode(v any) error {
	return c.decodeFunc(v)
}

func (c *StreamingConn) Encode(v any) error {
	return c.encodeFunc(v)
}

func (c *StreamingConn) Flush() error {
	return nil
}

func (c *StreamingConn) GetAuthInformation() *srpc.AuthInformation {
	return c.authInfo
}

func (c *StreamingConn) Username() string {
	if c.authInfo == nil {
		return ""
	}
	return c.authInfo.Username
}

func (c *StreamingConn) Read(p []byte) (n int, err error) {
	if c.readFunc == nil {
		return 0, status.Error(codes.Unimplemented, "Read not implemented for this RPC")
	}
	return c.readFunc(p)
}

func (c *StreamingConn) Peek(n int) ([]byte, error) {
	if c.peekFunc == nil {
		return nil, status.Error(codes.Unimplemented, "Peek not implemented for this RPC")
	}
	return c.peekFunc(n)
}
