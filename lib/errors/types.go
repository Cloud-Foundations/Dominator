package errors

func (e *AlreadyExistsError) errorString() string {
	if e.ID != "" {
		return e.Resource + " " + e.ID + " already exists"
	}
	return e.Resource + " already exists"
}

func (e *DeadlineExceededError) errorString() string {
	if e.Timeout != "" {
		return e.Operation + " exceeded deadline of " + e.Timeout
	}
	return e.Operation + " exceeded deadline"
}

func (e *FailedPreconditionError) errorString() string {
	if e.Reason != "" {
		return e.Resource + " " + e.State + ": " + e.Reason
	}
	return e.Resource + " " + e.State
}

func (e *InternalError) errorString() string {
	if e.Message != "" {
		return "internal error: " + e.Message
	}
	return "internal error"
}

func (e *InvalidArgumentError) errorString() string {
	if e.Reason != "" {
		return "invalid argument " + e.Argument + ": " + e.Reason
	}
	return "invalid argument: " + e.Argument
}

func (e *NotFoundError) errorString() string {
	if e.ID != "" {
		return e.Resource + " " + e.ID + " not found"
	}
	return e.Resource + " not found"
}

func (e *PermissionDeniedError) errorString() string {
	if e.Reason != "" {
		return "permission denied: " + e.Action + " on " + e.Resource + ": " +
			e.Reason
	}
	return "permission denied: " + e.Action + " on " + e.Resource
}

func (e *ResourceExhaustedError) errorString() string {
	if e.Reason != "" {
		return e.Resource + " exhausted: " + e.Reason
	}
	return e.Resource + " exhausted"
}

func (e *UnauthenticatedError) errorString() string {
	if e.Message != "" {
		return "unauthenticated: " + e.Message
	}
	return "unauthenticated"
}

func (e *UnavailableError) errorString() string {
	if e.Reason != "" {
		return e.Service + " unavailable: " + e.Reason
	}
	return e.Service + " unavailable"
}

func (e *UnimplementedError) errorString() string {
	if e.Operation != "" {
		return e.Operation + " not implemented"
	}
	return "not implemented"
}
