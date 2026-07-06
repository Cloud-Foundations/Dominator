package grpc

import "google.golang.org/grpc/codes"

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
