package errors

import "google.golang.org/grpc/codes"

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return e.Resource + " " + e.ID + " not found"
	}
	return e.Resource + " not found"
}

func (e *NotFoundError) GrpcCode() codes.Code { return codes.NotFound }

func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

type PermissionDeniedError struct {
	Resource string
	Action   string
	Reason   string
}

func (e *PermissionDeniedError) Error() string {
	if e.Reason != "" {
		return "permission denied: " + e.Action + " on " + e.Resource + ": " + e.Reason
	}
	return "permission denied: " + e.Action + " on " + e.Resource
}

func (e *PermissionDeniedError) GrpcCode() codes.Code { return codes.PermissionDenied }

func NewPermissionDeniedError(resource, action, reason string) *PermissionDeniedError {
	return &PermissionDeniedError{Resource: resource, Action: action, Reason: reason}
}

type UnauthenticatedError struct {
	Message string
}

func (e *UnauthenticatedError) Error() string {
	if e.Message != "" {
		return "unauthenticated: " + e.Message
	}
	return "unauthenticated"
}

func (e *UnauthenticatedError) GrpcCode() codes.Code { return codes.Unauthenticated }

func NewUnauthenticatedError(message string) *UnauthenticatedError {
	return &UnauthenticatedError{Message: message}
}

type AlreadyExistsError struct {
	Resource string
	ID       string
}

func (e *AlreadyExistsError) Error() string {
	if e.ID != "" {
		return e.Resource + " " + e.ID + " already exists"
	}
	return e.Resource + " already exists"
}

func (e *AlreadyExistsError) GrpcCode() codes.Code { return codes.AlreadyExists }

func NewAlreadyExistsError(resource, id string) *AlreadyExistsError {
	return &AlreadyExistsError{Resource: resource, ID: id}
}

type InvalidArgumentError struct {
	Argument string
	Reason   string
}

func (e *InvalidArgumentError) Error() string {
	if e.Reason != "" {
		return "invalid argument " + e.Argument + ": " + e.Reason
	}
	return "invalid argument: " + e.Argument
}

func (e *InvalidArgumentError) GrpcCode() codes.Code { return codes.InvalidArgument }

func NewInvalidArgumentError(argument, reason string) *InvalidArgumentError {
	return &InvalidArgumentError{Argument: argument, Reason: reason}
}

type FailedPreconditionError struct {
	Resource string
	State    string
	Reason   string
}

func (e *FailedPreconditionError) Error() string {
	if e.Reason != "" {
		return e.Resource + " " + e.State + ": " + e.Reason
	}
	return e.Resource + " " + e.State
}

func (e *FailedPreconditionError) GrpcCode() codes.Code { return codes.FailedPrecondition }

func NewFailedPreconditionError(resource, state, reason string) *FailedPreconditionError {
	return &FailedPreconditionError{Resource: resource, State: state, Reason: reason}
}

type UnavailableError struct {
	Service string
	Reason  string
}

func (e *UnavailableError) Error() string {
	if e.Reason != "" {
		return e.Service + " unavailable: " + e.Reason
	}
	return e.Service + " unavailable"
}

func (e *UnavailableError) GrpcCode() codes.Code { return codes.Unavailable }

func NewUnavailableError(service, reason string) *UnavailableError {
	return &UnavailableError{Service: service, Reason: reason}
}

type DeadlineExceededError struct {
	Operation string
	Timeout   string
}

func (e *DeadlineExceededError) Error() string {
	if e.Timeout != "" {
		return e.Operation + " exceeded deadline of " + e.Timeout
	}
	return e.Operation + " exceeded deadline"
}

func (e *DeadlineExceededError) GrpcCode() codes.Code { return codes.DeadlineExceeded }

func NewDeadlineExceededError(operation, timeout string) *DeadlineExceededError {
	return &DeadlineExceededError{Operation: operation, Timeout: timeout}
}

type ResourceExhaustedError struct {
	Resource string
	Reason   string
}

func (e *ResourceExhaustedError) Error() string {
	if e.Reason != "" {
		return e.Resource + " exhausted: " + e.Reason
	}
	return e.Resource + " exhausted"
}

func (e *ResourceExhaustedError) GrpcCode() codes.Code { return codes.ResourceExhausted }

func NewResourceExhaustedError(resource, reason string) *ResourceExhaustedError {
	return &ResourceExhaustedError{Resource: resource, Reason: reason}
}

type InternalError struct {
	Message string
}

func (e *InternalError) Error() string {
	if e.Message != "" {
		return "internal error: " + e.Message
	}
	return "internal error"
}

func (e *InternalError) GrpcCode() codes.Code { return codes.Internal }

func NewInternalError(message string) *InternalError {
	return &InternalError{Message: message}
}

type UnimplementedError struct {
	Operation string
}

func (e *UnimplementedError) Error() string {
	if e.Operation != "" {
		return e.Operation + " not implemented"
	}
	return "not implemented"
}

func (e *UnimplementedError) GrpcCode() codes.Code { return codes.Unimplemented }

func NewUnimplementedError(operation string) *UnimplementedError {
	return &UnimplementedError{Operation: operation}
}
