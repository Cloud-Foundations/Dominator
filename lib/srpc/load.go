package srpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

func loadCertificates(directory string) ([]tls.Certificate, error) {
	dir, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	dir.Close()
	certs := make([]tls.Certificate, 0, len(names)/2)
	now := time.Now()
	for _, keyName := range names {
		var certName string
		if keyName == "key.pem" {
			_, ok := stringutil.ConvertListToMap(names, false)["cert.pem"]
			if !ok {
				continue
			}
			certName = "cert.pem"
		} else if !strings.HasSuffix(keyName, ".key") {
			continue
		} else {
			certName = keyName[:len(keyName)-3] + "cert"
		}
		cert, err := tls.LoadX509KeyPair(
			filepath.Join(directory, certName),
			filepath.Join(directory, keyName))
		if err != nil {
			return nil, fmt.Errorf("unable to load keypair: %s", err)
		}
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, err
		}
		if notYet := x509Cert.NotBefore.Sub(now); notYet > 0 {
			return nil, fmt.Errorf("%s will not be valid for %s",
				certName, format.Duration(notYet))
		}
		if expired := now.Sub(x509Cert.NotAfter); expired > 0 {
			return nil, fmt.Errorf("%s expired %s ago",
				certName, format.Duration(expired))
		}
		cert.Leaf = x509Cert
		certs = append(certs, cert)
	}
	if len(certs) < 1 {
		return nil, nil
	}
	// The first entries are tried first when doing the TLS handshake, so sort
	// the list of certificates to prefer "better" ones.
	// First pass: sort list so that certificates with the longest remaining
	// lifetime are listed first.
	sort.Slice(certs, func(leftIndex, rightIndex int) bool {
		return certs[leftIndex].Leaf.NotAfter.After(
			certs[rightIndex].Leaf.NotAfter)
	})
	// Second pass: sort list so that certificates with the most permitted
	// methods are listed first.
	sort.SliceStable(certs, func(leftIndex, rightIndex int) bool {
		leftMethods, _ := x509util.GetPermittedMethods(certs[leftIndex].Leaf)
		rightMethods, _ := x509util.GetPermittedMethods(certs[rightIndex].Leaf)
		if _, leftIsAdmin := leftMethods["*.*"]; leftIsAdmin {
			if _, rightIsAdmin := rightMethods["*.*"]; !rightIsAdmin {
				return true
			}
		}
		return len(leftMethods) > len(rightMethods)
	})
	return certs, nil
}
