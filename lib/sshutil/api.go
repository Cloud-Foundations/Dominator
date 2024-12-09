package sshutil

import (
	"golang.org/x/crypto/ssh"
)

// ParseCertificate will parse a base64-encoded SSH certificate, where the first
// field contains the certificate type and the second field contains the
// certificate, followed by an optional comment.
// The certificate type, certificate and comment (nil if missing) are returned
// on success, else an error is returned.
func ParseCertificate(input []byte) ([]byte, *ssh.Certificate, []byte, error) {
	return parseCertificate(input)
}
