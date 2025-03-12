package sshutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

func parseCertificate(input []byte) ([]byte, *ssh.Certificate, []byte, error) {
	fields := bytes.Fields(input)
	if len(fields) < 2 {
		return nil, nil, nil, fmt.Errorf("insufficient fields")
	}
	var comment []byte
	if len(fields) > 2 {
		comment = fields[2]
	}
	reader := bytes.NewReader(fields[1])
	decoder := base64.NewDecoder(base64.StdEncoding, reader)
	decoded, err := io.ReadAll(decoder)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error decoding: %s", err)
	}
	pubkey, err := ssh.ParsePublicKey(decoded)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("failed to parse SSH public key: %s", err)
	}
	cert, ok := pubkey.(*ssh.Certificate)
	if !ok {
		return nil, nil, nil,
			fmt.Errorf("SSH public key is not a certificate, type: %S", pubkey)
	}
	return fields[0], cert, comment, nil
}
