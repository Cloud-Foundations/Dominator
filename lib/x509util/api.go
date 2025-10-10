/*
Package x509util provides utility functions to process X509 certificates.
*/
package x509util

import (
	"crypto/x509"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

// GetGroupList decodes the list of groups in the certificate.
// The group names are returned as keys in a map. An empty map indicates no
// group listed. If there is a problem parsing the information an error is
// returned.
func GetGroupList(cert *x509.Certificate) (map[string]struct{}, error) {
	return getList(cert, constants.GroupListOID)
}

// GetPermittedMethods decodes the list of permitted methods in the certificate.
// The permitted methods are returned as keys in a map. An empty map indicates
// no methods are permitted. If there is a problem parsing the information an
// error is returned.
func GetPermittedMethods(cert *x509.Certificate) (map[string]struct{}, error) {
	return getPermittedMethods(cert)
}

// GetUsername decodes the username for whom the certificate was granted. It
// attests the identity of the user.
func GetUsername(cert *x509.Certificate) (string, error) {
	return getUsername(cert)
}

// LoadCertificatePEM will decode the certificate found in the specified file
// containing PEM data. It returns the certificate and PEM headers if found,
// else an error.
// If there are extra data a message is logged.
func LoadCertificatePEM(filename string,
	logger log.DebugLogger) (*x509.Certificate, map[string]string, error) {
	return loadCertificatePEM(filename, logger)
}

// LoadCertificatePEMs will decode all the certificates found in the specified
// file containing PEM data. It returns a slice of certificates and a slice of
// PEM headers on success, else an error.
// If no certificates are found, an empty slice is returned.
// If no PEM headers are found in any PEM block, an empty slice is returned.
func LoadCertificatePEMs(filename string) (
	[]*x509.Certificate, []map[string]string, error) {
	return loadCertificatePEMs(filename)
}

// ParseCertificatePEM will decode the certificate found in the specified PEM
// data. It returns the certificate and PEM headers if found, else an error.
// If there are extra data a message is logged.
func ParseCertificatePEM(pemData []byte,
	logger log.DebugLogger) (*x509.Certificate, map[string]string, error) {
	return parseCertificatePEM(pemData, logger)
}

// ParseCertificatePEMs will decode all the certificates found in the specified
// PEM data. It returns a slice of certificates and a slice of PEM headers on
// success, else an error.
// If no certificates are found, an empty slice is returned.
// If no PEM headers are found in any PEM block, an empty slice is returned.
func ParseCertificatePEMs(pemData []byte) (
	[]*x509.Certificate, []map[string]string, error) {
	return parseCertificatePEMs(pemData)
}
