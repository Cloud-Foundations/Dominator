package sshutil

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"golang.org/x/crypto/ssh"
)

const (
	algorithmUnknown = "UNKNOWN AlgorithmType"
)

type validIntervalType struct {
	notAfter  time.Time
	notBefore time.Time
}

var (
	algorithmToText = map[AlgorithmType]string{
		AlgorithmEd25519: "Ed25519",
		AlgorithmRsa:     "RSA",
	}
	textToAlgorithmType map[string]AlgorithmType
)

func init() {
	textToAlgorithmType = make(map[string]AlgorithmType, len(algorithmToText))
	for algorithm, text := range algorithmToText {
		textToAlgorithmType[text] = algorithm
	}
}

func getFromMetadata(path string) ([]byte, error) {
	client := http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(constants.MetadataUrl + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func makeValidInterval(cert *ssh.Certificate) *validIntervalType {
	return &validIntervalType{
		notAfter:  time.Unix(int64(cert.ValidBefore), 0),
		notBefore: time.Unix(int64(cert.ValidAfter), 0),
	}
}

func newMetadataFetcher(config MetadataFetcherConfig,
	params MetadataFetcherParams) (*MetadataFetcher, error) {
	if config.Algorithm == AlgorithmNone {
		return nil, nil
	}
	keyfile, err := config.Algorithm.keyfile()
	if err != nil {
		return nil, err
	}
	if config.Directory == "" {
		config.Directory = filepath.Join(os.Getenv("HOME"), ".ssh")
	}
	if config.CertificateFilename == "" {
		config.CertificateFilename = filepath.Join(config.Directory,
			keyfile+"-cert.pub")
	}
	if config.KeyFilename == "" {
		config.KeyFilename = filepath.Join(config.Directory, keyfile)
	}
	fetcher := &MetadataFetcher{
		certPath: config.Algorithm.certPath(),
		config:   config,
		keyPath:  config.Algorithm.keyPath(),
		params:   params,
	}
	if certData, err := os.ReadFile(config.CertificateFilename); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else if _, cert, _, err := ParseCertificate(certData); err != nil {
		return nil, err
	} else {
		validInterval := makeValidInterval(cert)
		params.Logger.Printf(
			"read SSH %s certificate for: \"%s\", expires at: %s (in %s)\n",
			config.Algorithm, cert.ValidPrincipals[0],
			validInterval.notAfter.Format(format.TimeFormatSeconds),
			format.Duration(time.Until(validInterval.notAfter)))
		if validInterval.isValid() {
			go fetcher.loop(validInterval)
			return fetcher, nil
		}
	}
	if validInterval, err := fetcher.fetch(); err != nil {
		return nil, err
	} else {
		go fetcher.loop(validInterval)
		return fetcher, nil
	}
}

func (algorithm AlgorithmType) certPath() string {
	switch algorithm {
	case AlgorithmRsa:
		return constants.MetadataIdentityRsaSshCert
	case AlgorithmEd25519:
		return constants.MetadataIdentityEd25519SshCert
	default:
		panic("unsupported algorithm")
	}
}

func (algorithm AlgorithmType) keyPath() string {
	switch algorithm {
	case AlgorithmRsa:
		return constants.MetadataIdentityRsaSshKey
	case AlgorithmEd25519:
		return constants.MetadataIdentityEd25519SshKey
	default:
		panic("unsupported algorithm")
	}
}

func (algorithm AlgorithmType) keyfile() (string, error) {
	switch algorithm {
	case AlgorithmRsa:
		return "id_rsa", nil
	case AlgorithmEd25519:
		return "id_ed25519", nil
	default:
		return "", errors.New("unsupported algorithm")
	}
}

func (algorithm AlgorithmType) string() string {
	if str, ok := algorithmToText[algorithm]; !ok {
		return algorithmUnknown
	} else {
		return str
	}
}

func (algorithm *AlgorithmType) unmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToAlgorithmType[txt]; ok {
		*algorithm = val
		return nil
	} else {
		return errors.New("unknown AlgorithmType: " + txt)
	}
}

func (f *MetadataFetcher) fetch() (*validIntervalType, error) {
	certData, err := getFromMetadata(f.certPath)
	if err != nil {
		return nil, err
	}
	keyData, err := getFromMetadata(f.keyPath)
	if err != nil {
		return nil, err
	}
	_, cert, _, err := ParseCertificate(certData)
	if err != nil {
		return nil, err
	}
	validInterval := makeValidInterval(cert)
	if !validInterval.isValid() {
		return nil, errors.New("fetched invalid certificate")
	}
	err = fsutil.CopyToFile(f.config.CertificateFilename,
		fsutil.PublicFilePerms, bytes.NewReader(certData), 0)
	if err != nil {
		return nil, err
	}
	err = fsutil.CopyToFile(f.config.KeyFilename, fsutil.PrivateFilePerms,
		bytes.NewReader(keyData), 0)
	if err != nil {
		return nil, err
	}
	f.params.Logger.Printf(
		"fetched SSH %s certificate for: \"%s\", expires at: %s (in %s)\n",
		f.config.Algorithm, cert.ValidPrincipals[0],
		validInterval.notAfter.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(validInterval.notAfter)))
	return validInterval, nil
}

func (f *MetadataFetcher) loop(vi *validIntervalType) {
	halftime := vi.notBefore.Add(vi.notAfter.Sub(vi.notBefore) >> 1)
	refresher := backoffdelay.NewRefresher(vi.notAfter, 0, 0)
	time.Sleep(time.Until(halftime))
	for ; true; refresher.Sleep() {
		if vi, err := f.fetch(); err != nil {
			f.params.Logger.Println(err)
		} else {
			refresher.SetDeadline(vi.notAfter)
		}
	}
}

func (vi *validIntervalType) isValid() bool {
	if time.Since(vi.notBefore) >= 0 &&
		time.Until(vi.notAfter) > time.Minute {
		return true
	}
	return false
}
