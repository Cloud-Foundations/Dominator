/*
Package setupclient assists in setting up TLS credentials for a client.

Package setupclient provides convenience functions for setting up a client
(tool) with TLS credentials.
*/
package setupclient

import (
	"flag"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

var (
	certDirectory = flag.String("certDirectory",
		path.Join(os.Getenv("HOME"), ".ssl"),
		"Name of directory containing user SSL certificates")
)

type Params struct {
	IgnoreMissingCerts bool // true: ignore if certs missing.
	Logger             log.DebugLogger
}

// GetCertDirectory returns the directory containing the client certificates.
func GetCertDirectory() string {
	return *certDirectory
}

// SetupTls loads zero or more client certificates from files and registers them
// with the lib/srpc package. The following command-line flags are registered
// with the standard flag package:
//
//	-certDirectory: Name of directory containing user SSL certificates
func SetupTls(ignoreMissingCerts bool) error {
	return setupTls(Params{IgnoreMissingCerts: ignoreMissingCerts})
}

// SetupTlsWithParsms loads zero or more client certificates from files and
// registers them with the lib/srpc package. The following command-line flags
// are registered with the standard flag package:
//
//	-certDirectory: Name of directory containing user SSL certificates
func SetupTlsWithParams(params Params) error {
	return setupTls(params)
}
