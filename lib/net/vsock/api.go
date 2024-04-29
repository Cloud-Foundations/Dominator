package vsock

// CheckVsockets returns nil if VSOCK sockets are supported, else it returns an
// error.
func CheckVsockets() error {
	return checkVsockets()
}

// GetContextID returns the local Context Identifier for the VSOCK socket
// address family. This may be a privileged operation.
func GetContextID() (uint32, error) {
	return getContextID()
}
