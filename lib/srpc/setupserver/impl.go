package setupserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

const dateTime = time.DateTime + " MST"

var (
	caFile = flag.String("CAfile", "/etc/ssl/CA.pem",
		"Name of file containing the root of trust for identity and methods")
	certFile = flag.String("certFile",
		filepath.Join("/etc/ssl", getDirname(), "cert.pem"),
		"Name of file containing the SSL certificate")
	identityCaFile = flag.String("identityCAfile", "/etc/ssl/IdentityCA.pem",
		"Name of file containing the root of trust for identity only")
	keyFile = flag.String("keyFile",
		filepath.Join("/etc/ssl", getDirname(), "key.pem"),
		"Name of file containing the SSL key")

	certGenerationError error
	generatedTlsCert    *tls.Certificate
	generateCertOnce    sync.Once
)

// CheckFile returns true if the file exists and has size greater than zero,
// else it returns false.
func checkFile(filename string, params Params) bool {
	if filename == "" {
		return false
	}
	if fi, err := os.Stat(filename); err != nil {
		if !os.IsNotExist(err) {
			params.Logger.Println(err)
		}
		return false
	} else if fi.Size() == 0 {
		return false
	}
	return true
}

func generateTlsCert(params Params) (*tls.Certificate, error) {
	generateCertOnce.Do(func() {
		generatedTlsCert, certGenerationError = generateTlsCertOnce(params)
	})
	return generatedTlsCert, certGenerationError
}

func generateTlsCertOnce(params Params) (*tls.Certificate, error) {
	startTime := time.Now()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	year := 365*24*time.Hour + 6*time.Hour // Factor in leap years.
	templateCert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    startTime,
		NotAfter:     startTime.Add(30 * year),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"any-server"},
	}
	x509CertDER, err := x509.CreateCertificate(rand.Reader, templateCert,
		templateCert, publicKey, privateKey)
	if err != nil {
		return nil, err
	}
	x509Certs, err := x509.ParseCertificates(x509CertDER)
	if err != nil {
		return nil, err
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{x509CertDER},
		Leaf:        x509Certs[0],
		PrivateKey:  privateKey,
	}
	params.Logger.Printf(
		"Generated self-signed webcert in %s, expires in 30 years\n",
		format.Duration(time.Since(startTime)))
	return tlsCert, nil
}

func getDirname() string {
	return filepath.Base(os.Args[0])
}

func loadClientCert(params Params) (*tls.Certificate, error) {
	// Load certificate and key.
	if !checkFile(*certFile, params) || !checkFile(*keyFile, params) {
		cert, err := srpc.LoadCertificatesFromMetadata(100*time.Millisecond,
			true, false)
		if err != nil {
			return nil, err
		}
		params.Logger.Debugln(0,
			"Loaded certifcate and key from metadata service")
		return cert, nil
	}
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			cert, e := srpc.LoadCertificatesFromMetadata(100*time.Millisecond,
				true, false)
			if e != nil {
				return nil, srpc.ErrorMissingCertificate
			}
			params.Logger.Debugln(0,
				"Loaded certifcate and key from metadata service")
			return cert, nil
		}
		return nil, fmt.Errorf("unable to load keypair: %s", err)
	}
	if cert.Leaf == nil {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, err
		}
		cert.Leaf = x509Cert
	}
	params.Logger.Debugf(0, "Loaded certifcate and key from: %s and %s\n",
		*certFile, *keyFile)
	return &cert, nil
}

func loadLoop(params Params, cert *x509.Certificate) {
	params.FailIfExpired = true
	refresher := newRefresher(cert)
	for {
		sleepInterval := refresher.WaitInterval()
		params.Logger.Printf("Certificate refetch at: %s (%s)\n",
			time.Now().Add(sleepInterval).Format(dateTime),
			format.Duration(sleepInterval))
		time.Sleep(sleepInterval)
		if c, err := setupTlsOnce(params); err != nil {
			params.Logger.Println(err)
		} else {
			cert = c
			refresher.SetDeadline(cert.NotAfter)
		}
	}
}

func newRefresher(cert *x509.Certificate) *backoffdelay.Refresher {
	if cert != nil {
		return backoffdelay.NewRefresher(cert.NotAfter, time.Minute, 0)
	}
	return backoffdelay.NewRefresher(time.Now().Add(time.Hour), time.Minute, 0)
}

func setupTls(params Params) error {
	if params.Logger == nil {
		params.Logger = nulllogger.New()
	}
	cert, err := setupTlsOnce(params)
	if err != nil {
		return err
	}
	go loadLoop(params, cert)
	return nil
}

func setupTlsOnce(params Params) (*x509.Certificate, error) {
	var x509Cert *x509.Certificate
	tlsCert, err := setupClientTlsOnce(params)
	if err != nil {
		if err != srpc.ErrorMissingCertificate {
			return nil, err
		}
		if params.ClientOnly {
			return nil, err
		}
		if !params.GenerateIfMissing {
			return nil, err
		}
		tlsCert, err = generateTlsCert(params)
	} else {
		x509Cert = tlsCert.Leaf
	}
	if err != nil {
		return nil, err
	}
	if !params.ClientOnly {
		if err := setupServerTlsOnce(params, tlsCert); err != nil {
			return nil, err
		}
	}
	return x509Cert, nil
}

func setupClientTlsOnce(params Params) (*tls.Certificate, error) {
	tlsCert, err := loadClientCert(params)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	x509Cert := tlsCert.Leaf
	if notYet := x509Cert.NotBefore.Sub(now); notYet > 0 {
		msg := fmt.Sprintf("%s will not be valid for %s",
			*certFile, format.Duration(notYet))
		if params.FailIfExpired {
			return nil, errors.New(msg)
		}
		params.Logger.Println(msg)
	} else if expired := now.Sub(x509Cert.NotAfter); expired > 0 {
		msg := fmt.Sprintf("%s expired %s ago",
			*certFile, format.Duration(expired))
		if params.FailIfExpired {
			return nil, errors.New(msg)
		}
		params.Logger.Println(msg)
	} else {
		params.Logger.Printf("Certificate expires at: %s (%s)\n",
			x509Cert.NotAfter.Local().Format(dateTime),
			format.Duration(time.Until(x509Cert.NotAfter)))
	}
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = append(clientConfig.Certificates, *tlsCert)
	srpc.RegisterClientTlsConfig(clientConfig)
	return tlsCert, nil
}

func setupServerTlsOnce(params Params, serverCert *tls.Certificate) error {
	if *caFile == "" {
		if params.PermitInsecure {
			params.Logger.Println(
				"no CA file specified: unauthenticated connections permitted")
			return nil
		}
		return srpc.ErrorMissingCA
	}
	caCertPool := x509.NewCertPool()
	identityCertPool := x509.NewCertPool()
	var earliestCertExpiration time.Time
	if certs, _, err := x509util.LoadCertificatePEMs(*caFile); err != nil {
		if os.IsNotExist(err) {
			if params.PermitInsecure {
				params.Logger.Printf(
					"%s not found: unauthenticated connections permitted\n",
					*caFile)
				return nil
			}
			return srpc.ErrorMissingCA
		}
		return fmt.Errorf("unable to load CA file: \"%s\": %s", *caFile, err)
	} else {
		for _, cert := range certs {
			caCertPool.AddCert(cert)
			identityCertPool.AddCert(cert)
			if earliestCertExpiration.IsZero() ||
				cert.NotAfter.Before(earliestCertExpiration) {
				earliestCertExpiration = cert.NotAfter
			}
		}
	}
	serverConfig := new(tls.Config)
	serverConfig.ClientAuth = tls.RequireAndVerifyClientCert
	serverConfig.MinVersion = tls.VersionTLS12
	serverConfig.ClientCAs = caCertPool
	if *identityCaFile != "" {
		certs, _, err := x509util.LoadCertificatePEMs(*identityCaFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("unable to load CA file: \"%s\": %s",
					*caFile, err)
			}
		} else {
			srpc.RegisterFullAuthCA(caCertPool)
			for _, cert := range certs {
				identityCertPool.AddCert(cert)
				if earliestCertExpiration.IsZero() ||
					cert.NotAfter.Before(earliestCertExpiration) {
					earliestCertExpiration = cert.NotAfter
				}
			}
			serverConfig.ClientCAs = identityCertPool
		}
	}
	serverConfig.Certificates = append(serverConfig.Certificates, *serverCert)
	srpc.RegisterServerTlsConfig(serverConfig, true)
	tricorder.RegisterMetric("/srpc/server/earliest-ca-expiration",
		&earliestCertExpiration, units.None,
		"expiration time of the CA which will expire the soonest")
	return nil
}
