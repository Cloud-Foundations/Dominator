package manager

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

func parseKeyPair(certPEM, keyPEM []byte) (*x509.Certificate, error) {
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if notYet := x509Cert.NotBefore.Sub(now); notYet > 0 {
		return nil,
			fmt.Errorf("cert will not be valid for %s", format.Duration(notYet))
	}
	if expired := now.Sub(x509Cert.NotAfter); expired > 0 {
		return nil, fmt.Errorf("cert expired %s ago", format.Duration(expired))
	}

	return x509Cert, nil
}

func validateIdentityKeyPair(certPEM, keyPEM []byte, username string) (
	string, time.Time, error) {
	x509Cert, err := parseKeyPair(certPEM, keyPEM)
	if err != nil {
		return "", time.Time{}, err
	}
	certUsername, err := x509util.GetUsername(x509Cert)
	if err != nil {
		return "", time.Time{}, err
	}
	if username == certUsername {
		return "", time.Time{}, fmt.Errorf("cannot give VM your own identity")
	}
	return certUsername, x509Cert.NotAfter, nil
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
