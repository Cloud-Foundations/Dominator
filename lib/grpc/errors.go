package grpc

import (
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CodedError is implemented by errors that provide a gRPC status code.
type CodedError interface {
	GrpcCode() codes.Code
}

var prefixMappings = []struct {
	prefix string
	code   codes.Code
}{
	{"unauthenticated:", codes.Unauthenticated},
	{"permission denied:", codes.PermissionDenied},
	{"precondition failed:", codes.FailedPrecondition},
	{"not found:", codes.NotFound},
	{"already exists:", codes.AlreadyExists},
	{"invalid argument:", codes.InvalidArgument},
	{"unavailable:", codes.Unavailable},
	{"deadline exceeded:", codes.DeadlineExceeded},
	{"resource exhausted:", codes.ResourceExhausted},
	{"internal:", codes.Internal},
}

var patternMappings = []struct {
	pattern string
	code    codes.Code
}{
	// Unauthenticated
	{"no authentication", codes.Unauthenticated},
	{"unauthenticated", codes.Unauthenticated},
	{"not authenticated", codes.Unauthenticated},
	{"authentication required", codes.Unauthenticated},
	// PermissionDenied
	{"permission denied", codes.PermissionDenied},
	{"access denied", codes.PermissionDenied},
	{"forbidden", codes.PermissionDenied},
	{"no access", codes.PermissionDenied},
	// NotFound
	{"not found", codes.NotFound},
	{"does not exist", codes.NotFound},
	// AlreadyExists
	{"already exists", codes.AlreadyExists},
	{"duplicate", codes.AlreadyExists},
	// InvalidArgument
	{"invalid", codes.InvalidArgument},
	{"malformed", codes.InvalidArgument},
	// Unavailable
	{"unavailable", codes.Unavailable},
	{"connection refused", codes.Unavailable},
	// DeadlineExceeded
	{"timeout", codes.DeadlineExceeded},
	{"timed out", codes.DeadlineExceeded},
}

// ErrorToStatus converts errors to gRPC status codes.
func ErrorToStatus(err error) error {
	if err == nil {
		return nil
	}
	var codedErr CodedError
	if errors.As(err, &codedErr) {
		return status.Error(codedErr.GrpcCode(), err.Error())
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	for _, m := range prefixMappings {
		if strings.HasPrefix(lower, m.prefix) {
			return status.Error(m.code, msg)
		}
	}
	for _, m := range patternMappings {
		if strings.Contains(lower, m.pattern) {
			return status.Error(m.code, msg)
		}
	}
	return status.Error(codes.Internal, msg)
}
