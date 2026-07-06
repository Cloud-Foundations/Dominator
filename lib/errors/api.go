package errors

import (
	"errors"

	"google.golang.org/grpc/codes"
)

// AlreadyExistsError indicates the resource already exists.
type AlreadyExistsError struct {
	Resource string
	ID       string
}

// DeadlineExceededError indicates an operation exceeded its deadline.
type DeadlineExceededError struct {
	Operation string
	Timeout   string
}

// FailedPreconditionError indicates a precondition on the resource failed.
type FailedPreconditionError struct {
	Resource string
	State    string
	Reason   string
}

// InternalError indicates an internal failure.
type InternalError struct {
	Message string
}

// InvalidArgumentError indicates an argument was invalid.
type InvalidArgumentError struct {
	Argument string
	Reason   string
}

// NotFoundError indicates the requested resource was not found.
type NotFoundError struct {
	Resource string
	ID       string
}

// PermissionDeniedError indicates the caller is not permitted to perform the
// action on the resource.
type PermissionDeniedError struct {
	Resource string
	Action   string
	Reason   string
}

// ResourceExhaustedError indicates a resource has been exhausted.
type ResourceExhaustedError struct {
	Resource string
	Reason   string
}

// UnauthenticatedError indicates the caller was not authenticated.
type UnauthenticatedError struct {
	Message string
}

// UnavailableError indicates a service is unavailable.
type UnavailableError struct {
	Service string
	Reason  string
}

// UnimplementedError indicates the operation is not implemented.
type UnimplementedError struct {
	Operation string
}

// ErrorToString returns the string representation of err or an empty string if
// err is nil.
func ErrorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// New returns a new error with the specified text, or nil if the text is empty.
func New(text string) error {
	if text == "" {
		return nil
	}
	return errors.New(text)
}

// NewAlreadyExistsError constructs an AlreadyExistsError.
func NewAlreadyExistsError(resource, id string) *AlreadyExistsError {
	return &AlreadyExistsError{Resource: resource, ID: id}
}

// NewDeadlineExceededError constructs a DeadlineExceededError.
func NewDeadlineExceededError(operation, timeout string) *DeadlineExceededError {
	return &DeadlineExceededError{Operation: operation, Timeout: timeout}
}

// NewFailedPreconditionError constructs a FailedPreconditionError.
func NewFailedPreconditionError(resource, state,
	reason string) *FailedPreconditionError {
	return &FailedPreconditionError{
		Resource: resource,
		State:    state,
		Reason:   reason,
	}
}

// NewInternalError constructs an InternalError.
func NewInternalError(message string) *InternalError {
	return &InternalError{Message: message}
}

// NewInvalidArgumentError constructs an InvalidArgumentError.
func NewInvalidArgumentError(argument, reason string) *InvalidArgumentError {
	return &InvalidArgumentError{Argument: argument, Reason: reason}
}

// NewNotFoundError constructs a NotFoundError.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// NewPermissionDeniedError constructs a PermissionDeniedError.
func NewPermissionDeniedError(resource, action,
	reason string) *PermissionDeniedError {
	return &PermissionDeniedError{
		Resource: resource,
		Action:   action,
		Reason:   reason,
	}
}

// NewResourceExhaustedError constructs a ResourceExhaustedError.
func NewResourceExhaustedError(resource,
	reason string) *ResourceExhaustedError {
	return &ResourceExhaustedError{Resource: resource, Reason: reason}
}

// NewUnauthenticatedError constructs an UnauthenticatedError.
func NewUnauthenticatedError(message string) *UnauthenticatedError {
	return &UnauthenticatedError{Message: message}
}

// NewUnavailableError constructs an UnavailableError.
func NewUnavailableError(service, reason string) *UnavailableError {
	return &UnavailableError{Service: service, Reason: reason}
}

// NewUnimplementedError constructs an UnimplementedError.
func NewUnimplementedError(operation string) *UnimplementedError {
	return &UnimplementedError{Operation: operation}
}

func (e *AlreadyExistsError) Error() string        { return e.errorString() }
func (e *AlreadyExistsError) GrpcCode() codes.Code { return codes.AlreadyExists }

func (e *DeadlineExceededError) Error() string { return e.errorString() }
func (e *DeadlineExceededError) GrpcCode() codes.Code {
	return codes.DeadlineExceeded
}

func (e *FailedPreconditionError) Error() string { return e.errorString() }
func (e *FailedPreconditionError) GrpcCode() codes.Code {
	return codes.FailedPrecondition
}

func (e *InternalError) Error() string        { return e.errorString() }
func (e *InternalError) GrpcCode() codes.Code { return codes.Internal }

func (e *InvalidArgumentError) Error() string { return e.errorString() }
func (e *InvalidArgumentError) GrpcCode() codes.Code {
	return codes.InvalidArgument
}

func (e *NotFoundError) Error() string        { return e.errorString() }
func (e *NotFoundError) GrpcCode() codes.Code { return codes.NotFound }

func (e *PermissionDeniedError) Error() string { return e.errorString() }
func (e *PermissionDeniedError) GrpcCode() codes.Code {
	return codes.PermissionDenied
}

func (e *ResourceExhaustedError) Error() string { return e.errorString() }
func (e *ResourceExhaustedError) GrpcCode() codes.Code {
	return codes.ResourceExhausted
}

func (e *UnauthenticatedError) Error() string { return e.errorString() }
func (e *UnauthenticatedError) GrpcCode() codes.Code {
	return codes.Unauthenticated
}

func (e *UnavailableError) Error() string        { return e.errorString() }
func (e *UnavailableError) GrpcCode() codes.Code { return codes.Unavailable }

func (e *UnimplementedError) Error() string        { return e.errorString() }
func (e *UnimplementedError) GrpcCode() codes.Code { return codes.Unimplemented }
