package x509util

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func loadCertificatePEM(filename string,
	logger log.DebugLogger) (*x509.Certificate, map[string]string, error) {
	pemData, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	return ParseCertificatePEM(pemData, logger)
}

func loadCertificatePEMs(filename string) (
	[]*x509.Certificate, []map[string]string, error) {
	pemData, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	return ParseCertificatePEMs(pemData)
}

func parseCertificatePEM(pemData []byte,
	logger log.DebugLogger) (*x509.Certificate, map[string]string, error) {
	block, rest := pem.Decode(pemData)
	if block == nil {
		return nil, nil, errors.New("error decoding PEM certificate")
	}
	if block.Type != "CERTIFICATE" {
		return nil, block.Headers,
			fmt.Errorf("unsupported certificate type: \"%s\"", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, block.Headers, err
	}
	if len(rest) > 0 {
		logger.Printf("%d extra bytes in certfile\n", len(rest))
	}
	return cert, block.Headers, nil
}

func parseCertificatePEMs(pemData []byte) (
	[]*x509.Certificate, []map[string]string, error) {
	var certs []*x509.Certificate
	var headersList []map[string]string
	var haveHeaders bool
	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		certBytes := block.Bytes
		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, nil, err
		}
		certs = append(certs, cert)
		headersList = append(headersList, block.Headers)
		if len(block.Headers) > 0 {
			haveHeaders = true
		}
	}
	if haveHeaders {
		return certs, headersList, nil
	}
	return certs, nil, nil
}
