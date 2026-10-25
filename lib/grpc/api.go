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
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
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

// DoNotUseMethodPowersMetadataKey is the metadata key (value "true") to opt
// out of method powers, like SRPC's doNotUseMethodPowers query parameter.
const DoNotUseMethodPowersMetadataKey = "donotusemethodpowers"

// AuthInfo holds the caller's authentication information.
type AuthInfo struct {
	authInformation *srpc.AuthInformation
}

// ServiceOptions configures authorisation for a gRPC service.
type ServiceOptions struct {
	PublicMethods          []string // Method names.
	UnauthenticatedMethods []string // Method names.
}

// AuthInfoFromContext returns the AuthInfo attached to ctx, or nil.
func AuthInfoFromContext(ctx context.Context) *AuthInfo {
	return authInfoFromContext(ctx)
}

// ContextWithAuthInfo returns a copy of ctx carrying authInfo.
func ContextWithAuthInfo(ctx context.Context,
	authInfo *AuthInfo) context.Context {
	return contextWithAuthInfo(ctx, authInfo)
}

// GetAuthInformation returns the authentication information or nil.
func (a *AuthInfo) GetAuthInformation() *srpc.AuthInformation {
	return a.getAuthInformation()
}

// IPToString returns the text form of ip, or the empty string if ip is empty.
func IPToString(ip net.IP) string {
	return ipToString(ip)
}

// MapToTags returns a copy of m as tags.Tags, or nil if m is nil.
func MapToTags(m map[string]string) tags.Tags {
	return mapToTags(m)
}

// RegisterServiceOptions registers public and unauthenticated methods for a
// service. Method names must match the SRPC method names exactly.
func RegisterServiceOptions(serviceName string, options ServiceOptions) {
	registerServiceOptions(serviceName, options)
}

// StringToIP parses s as an IP address. It returns a nil address and no error
// if s is empty. It returns an invalid argument error if s is malformed.
func StringToIP(s string) (net.IP, error) {
	return stringToIP(s)
}

// StringToMAC parses s as a MAC address. It returns a nil address and no error
// if s is empty. It returns an invalid argument error if s is malformed.
func StringToMAC(s string) (net.HardwareAddr, error) {
	return stringToMAC(s)
}

// StreamAuthInterceptor authenticates and authorises streaming RPCs.
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return streamAuthInterceptor(srv, ss, info, handler)
}

// TagsToMap returns a copy of t as a map, or nil if t is nil.
func TagsToMap(t tags.Tags) map[string]string {
	return tagsToMap(t)
}

// UnaryAuthInterceptor authenticates and authorises unary RPCs.
func UnaryAuthInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return unaryAuthInterceptor(ctx, req, info, handler)
}
