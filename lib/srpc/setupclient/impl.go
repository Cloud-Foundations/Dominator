package setupclient

import (
	"crypto/tls"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func loadCerts() ([]tls.Certificate, error) {
	if *certDirectory == "" {
		cert, err := srpc.LoadCertificatesFromMetadata(100*time.Millisecond,
			false, true)
		if err != nil {
			return nil, err
		}
		if cert == nil {
			return nil, nil
		}
		return []tls.Certificate{*cert}, nil
	}
	// Load certificates.
	certs, err := srpc.LoadCertificates(*certDirectory)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if certs != nil {
		return certs, nil
	}
	cert, err := srpc.LoadCertificatesFromMetadata(100*time.Millisecond, false,
		true)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, nil
	}
	return []tls.Certificate{*cert}, nil
}

func setupTls(ignoreMissingCerts bool) error {
	certs, err := loadCerts()
	if err != nil {
		return err
	}
	if certs == nil {
		if ignoreMissingCerts {
			return nil
		}
		return srpc.ErrorMissingCertificate
	}
	// Setup client.
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = certs
	srpc.RegisterClientTlsConfig(clientConfig)
	return nil
}
