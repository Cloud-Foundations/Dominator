/*
Package grpc provides gRPC server-side helpers for Dominator services:
authentication and authorisation interceptors that reuse SRPC's
authorisation logic, and a helper to convert Go errors into gRPC status
errors.

A gRPC server registers UnaryAuthInterceptor and StreamAuthInterceptor when
constructing the server, and registers per-service public/unauthenticated
method options via RegisterServiceOptions. gRPC method names of the form
"/package.Service/Method" are stripped to the SRPC form "Service.Method"
before being passed to the shared authorisation check, so a single set of
permitted-method entries in the caller's X509 client certificate governs
access over both protocols. Method names in the gRPC service definition
must therefore match the SRPC method names exactly.
*/
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

// CodedError is implemented by errors that provide a gRPC status code.
type CodedError interface {
	GrpcCode() codes.Code
}

// ErrorToStatus converts err into a gRPC status error. Errors implementing
// CodedError use their declared code; otherwise the message is matched against
// a table of prefixes and substrings, falling back to codes.Internal.
func ErrorToStatus(err error) error {
	return errorToStatus(err)
}

// DoNotUseMethodPowersMetadataKey is the gRPC request metadata key used to
// opt out of method powers on a per-RPC basis. Clients that want to opt out
// attach this key with the value "true"; this is the gRPC equivalent of
// SRPC's doNotUseMethodPowers query parameter. gRPC lower-cases metadata
// keys on the wire, so the header must be sent in lower case.
const DoNotUseMethodPowersMetadataKey = "donotusemethodpowers"

// Conn holds authentication information for gRPC handlers.
type Conn struct {
	authInfo          *srpc.AuthInformation
	permittedMethods  map[string]struct{}
	allowMethodPowers bool
}

// GetAuthInformation returns the authentication information or nil.
func (c *Conn) GetAuthInformation() *srpc.AuthInformation {
	if c == nil {
		return nil
	}
	return c.authInfo
}

// GetPermittedMethods returns the caller's permitted method set, or nil.
func (c *Conn) GetPermittedMethods() map[string]struct{} {
	if c == nil {
		return nil
	}
	return c.permittedMethods
}

// AllowMethodPowers reports whether the caller has opted in to method
// powers. See DoNotUseMethodPowersMetadataKey for how clients opt out.
func (c *Conn) AllowMethodPowers() bool {
	if c == nil {
		return false
	}
	return c.allowMethodPowers
}

// ConnFromContext returns the *Conn that the auth interceptors attached to
// ctx, or nil if ctx has not been passed through an auth interceptor.
// Handlers call this to obtain the caller's authentication and
// authorisation information.
func ConnFromContext(ctx context.Context) *Conn {
	if v := ctx.Value(connKey); v != nil {
		return v.(*Conn)
	}
	return nil
}

// ContextWithConn returns a derived context carrying conn. Handlers retrieve
// conn via ConnFromContext.
func ContextWithConn(ctx context.Context, conn *Conn) context.Context {
	return context.WithValue(ctx, connKey, conn)
}

// ServiceOptions configures authorisation for a gRPC service.
type ServiceOptions struct {
	PublicMethods          []string // Method names.
	UnauthenticatedMethods []string // Method names.
}

// RegisterServiceOptions registers public and unauthenticated methods for a
// service. Method names must match the SRPC method names exactly.
func RegisterServiceOptions(serviceName string, options ServiceOptions) {
	allMethods := make(map[string]struct{})
	for _, method := range options.PublicMethods {
		fullMethod := "/" + serviceName + "/" + method
		publicMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	for _, method := range options.UnauthenticatedMethods {
		fullMethod := "/" + serviceName + "/" + method
		unauthenticatedMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	registerMethodMetrics(serviceName, allMethods)
}

// UnaryAuthInterceptor handles authentication and authorisation for unary RPCs.
func UnaryAuthInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx, err := authoriseRequest(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}
	recordCallStart()
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			recordPanic()
			panic(r)
		}
	}()
	resp, err := handler(ctx, req)
	recordCallEnd(info.FullMethod, startTime, err)
	return resp, err
}

// StreamAuthInterceptor handles authentication and authorisation for streaming RPCs.
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, err := authoriseRequest(ss.Context(), info.FullMethod)
	if err != nil {
		return err
	}
	wrapped := &wrappedStream{ServerStream: ss, ctx: ctx}
	recordCallStart()
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			recordPanic()
			panic(r)
		}
	}()
	err = handler(srv, wrapped)
	recordCallEnd(info.FullMethod, startTime, err)
	return err
}
