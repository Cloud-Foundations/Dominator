package sshutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"golang.org/x/crypto/ssh"
)

const (
	AlgorithmNone AlgorithmType = iota
	AlgorithmEd25519
	AlgorithmRsa
)

type AlgorithmType uint

type MetadataFetcher struct {
	certPath string
	config   MetadataFetcherConfig
	keyPath  string
	params   MetadataFetcherParams
}

type MetadataFetcherConfig struct {
	// Algorithm specifies the public key algorithm. Default is none.
	Algorithm AlgorithmType

	// CertificateFilename specifies the name of the file to write the
	// certificate to. The default is "id_rsa" for the RSA algorithm and
	// "id_ed25519" for the Ed25519 algorithm.
	CertificateFilename string

	// Directory specifies the directory to write the certificate and key to.
	// The default is "$HOME/.ssh"
	Directory string

	// KeyFilename specifies the name of the file to write the private key to.
	// The default is "id_rsa-cert.pub" for the RSA algorithm and
	// "id_ed25519-cert.pub" for the Ed25519 algorithm.
	KeyFilename string
}

type MetadataFetcherParams struct {
	Logger log.DebugLogger
}

func (algorithm AlgorithmType) String() string {
	return algorithm.string()
}

func (algorithm *AlgorithmType) UnmarshalText(text []byte) error {
	return algorithm.unmarshalText(text)
}

// ParseCertificate will parse a base64-encoded SSH certificate, where the first
// field contains the certificate type and the second field contains the
// certificate, followed by an optional comment.
// The certificate type, certificate and comment (nil if missing) are returned
// on success, else an error is returned.
func ParseCertificate(input []byte) ([]byte, *ssh.Certificate, []byte, error) {
	return parseCertificate(input)
}

// NewMetadataFetcher will create a goroutine which will periodically load an
// SSH certificate and key from the SmallStack Metadata Service and write it to
// a local file. It will return an error if there is no valid certificate
// available from the file or the Metadata service.
// If no algorithm is specified, nil,nil is returned.
func NewMetadataFetcher(config MetadataFetcherConfig,
	params MetadataFetcherParams) (*MetadataFetcher, error) {
	return newMetadataFetcher(config, params)
}
