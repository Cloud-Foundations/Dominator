package vsock

// GetContextID returns the local Context Identifier for the VSOCK socket
// address family. This may be a privileged operation.
func GetContextID() (uint32, error) {
	return getContextID()
}
