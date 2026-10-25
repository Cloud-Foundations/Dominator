/*
Package server serves SRPC, HTML and gRPC from a single TCP port.

Connections that start with a TLS handshake are terminated with the SRPC server
TLS config, so that gRPC clients can connect with TLS and HTTP/2 from the first
byte. Other connections are passed through untouched, so that SRPC's plain
HTTP CONNECT and in-band TLS upgrade keep working. Requests with a gRPC content
type are then routed to the gRPC handler and all other requests to the HTTP
handler.
*/
package server

import (
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

// Config holds the settings of Start.
type Config struct {
	Port uint
}

// Params holds the other arguments of Start.
type Params struct {
	// If nil: the port is served as an SRPC server serves it without this
	// package, with plain HTTP only. Typically a *grpc.Server.
	GrpcHandler http.Handler
	// If nil: http.DefaultServeMux, where SRPC registers its handlers. A
	// non-nil handler must delegate SRPC requests to http.DefaultServeMux.
	HttpHandler http.Handler
	Logger      log.DebugLogger
}

// Start serves the port until it fails, returning the error. If an SRPC server
// TLS config has been registered with srpc.RegisterServerTlsConfig, gRPC is
// served over TLS, otherwise over cleartext HTTP/2.
func Start(config Config, params Params) error {
	return start(config, params)
}
