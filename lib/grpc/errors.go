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

	prefixMappings := []struct {
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

	for _, mapping := range prefixMappings {
		if strings.HasPrefix(lower, mapping.prefix) {
			return status.Error(mapping.code, msg)
		}
	}

	patternMappings := []struct {
		patterns []string
		code     codes.Code
	}{
		{[]string{"no authentication", "unauthenticated", "not authenticated", "authentication required"}, codes.Unauthenticated},
		{[]string{"permission denied", "access denied", "forbidden", "no access"}, codes.PermissionDenied},
		{[]string{"not found", "does not exist"}, codes.NotFound},
		{[]string{"already exists", "duplicate"}, codes.AlreadyExists},
		{[]string{"invalid", "malformed"}, codes.InvalidArgument},
		{[]string{"unavailable", "connection refused"}, codes.Unavailable},
		{[]string{"timeout", "timed out"}, codes.DeadlineExceeded},
	}

	for _, mapping := range patternMappings {
		if containsAny(msg, mapping.patterns) {
			return status.Error(mapping.code, msg)
		}
	}

	return status.Error(codes.Internal, msg)
}

func containsAny(s string, substrings []string) bool {
	lower := strings.ToLower(s)
	for _, substr := range substrings {
		if strings.Contains(lower, substr) {
			return true
		}
	}
	return false
}
