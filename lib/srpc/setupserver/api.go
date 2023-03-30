/*
	Package setupserver assists in setting up TLS credentials for a server.

	Package setupserver provides convenience functions for setting up a server
	with TLS credentials.

	The package loads client and server certificates from files and registers
	them with the lib/srpc package. The following command-line flags are
	registered with the standard flag package:
	  -caFile:   Name of file containing the root of trust
	  -certFile: Name of file containing the SSL certificate
	  -keyFile:  Name of file containing the SSL key
*/
package setupserver

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type Params struct {
	ClientOnly    bool // If true, only register client certificate and key.
	FailIfExpired bool // If true, fail if certificate not yet valid or expired.
	Logger        log.DebugLogger
}

func SetupTls() error {
	return setupTls(Params{})
}

func SetupTlsClientOnly() error {
	return setupTls(Params{ClientOnly: true})
}

func SetupTlsWithParams(params Params) error {
	return setupTls(params)
}
