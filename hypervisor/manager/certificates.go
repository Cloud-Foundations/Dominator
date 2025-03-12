package manager

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

func decodeCert(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("error decoding PEM certificate")
	}
	if block.Type != "CERTIFICATE" {
		return nil,
			fmt.Errorf("unsupported certificate type: \"%s\"", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func parseKeyPair(certPEM, keyPEM []byte) (*tls.Certificate, string, error) {
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, "", err
	}
	x509Cert := tlsCert.Leaf
	if x509Cert == nil {
		x509Cert, err = x509.ParseCertificate(tlsCert.Certificate[0])
		if err != nil {
			return nil, "", err
		}
		tlsCert.Leaf = x509Cert
	}
	now := time.Now()
	if notYet := x509Cert.NotBefore.Sub(now); notYet > 0 {
		return nil, "",
			fmt.Errorf("cert will not be valid for %s", format.Duration(notYet))
	}
	if expired := now.Sub(x509Cert.NotAfter); expired > 0 {
		return nil, "",
			fmt.Errorf("cert expired %s ago", format.Duration(expired))
	}
	username, err := x509util.GetUsername(x509Cert)
	if err != nil {
		return nil, "", err
	}
	return &tlsCert, username, nil
}

func validateIdentityKeyPair(certPEM, keyPEM []byte, username string) (
	*tls.Certificate, string, error) {
	tlsCert, certUsername, err := parseKeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, "", err
	}
	if username == certUsername {
		return nil, "", fmt.Errorf("cannot give VM your own identity")
	}
	return tlsCert, certUsername, nil
}

func writeKeyPair(certPEM, keyPEM []byte,
	certFilename, keyFilename string) error {
	if len(certPEM) < 1 || len(keyPEM) < 1 {
		return nil
	}
	err := ioutil.WriteFile(certFilename, certPEM, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(keyFilename, keyPEM, fsutil.PrivateFilePerms)
	if err != nil {
		return err
	}
	return nil
}
