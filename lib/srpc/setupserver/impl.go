package setupserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

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
)

func getDirname() string {
	return filepath.Base(os.Args[0])
}

func loadCerts(filename string) ([]*x509.Certificate, error) {
	pemData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var certs []*x509.Certificate
	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}
		certBytes := block.Bytes
		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func getSleepInterval(cert *x509.Certificate) time.Duration {
	day := 24 * time.Hour
	week := 7 * day
	lifetime := cert.NotAfter.Sub(cert.NotBefore)
	refreshIn := time.Until(cert.NotBefore.Add(7 * lifetime >> 3))
	if refreshIn > 0 {
		return refreshIn
	}
	expiresIn := time.Until(cert.NotAfter)
	if expiresIn > 2*week {
		return week
	} else if expiresIn > 2*day {
		return day
	} else if expiresIn > 2*time.Hour {
		return time.Hour
	} else if expiresIn > 2*time.Minute {
		return time.Minute
	} else {
		return 5 * time.Second
	}
}

func loadClientCert(params Params) (*tls.Certificate, error) {
	// Load certificate and key.
	if *certFile == "" || *keyFile == "" {
		cert, err := srpc.LoadCertificatesFromMetadata(100*time.Millisecond,
			true, false)
		if err != nil {
			return nil, err
		}
		params.Logger.Debugln(0,
			"Loaded certifcate and key from metadata service\n")
		return cert, nil
	}
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			cert, e := srpc.LoadCertificatesFromMetadata(100*time.Millisecond,
				true, false)
			if e != nil {
				return nil, err
			}
			params.Logger.Debugln(0,
				"Loaded certifcate and key from metadata service\n")
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
	for {
		time.Sleep(getSleepInterval(cert))
		if c, err := setupTlsOnce(params); err != nil {
			params.Logger.Println(err)
		} else {
			cert = c
		}
	}
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
	// Setup client.
	tlsCert, err := loadClientCert(params)
	if err != nil {
		return nil, fmt.Errorf("unable to load keypair: %s", err)
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
		params.Logger.Debugf(0, "Certificate expires at: %s (%s)\n",
			x509Cert.NotAfter.Local(),
			format.Duration(time.Until(x509Cert.NotAfter)))
	}
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = append(clientConfig.Certificates, *tlsCert)
	srpc.RegisterClientTlsConfig(clientConfig)
	if !params.ClientOnly {
		if *caFile == "" {
			return nil, srpc.ErrorMissingCA
		}
		caCertPool := x509.NewCertPool()
		identityCertPool := x509.NewCertPool()
		var earliestCertExpiration time.Time
		if certs, err := loadCerts(*caFile); err != nil {
			if os.IsNotExist(err) {
				return nil, srpc.ErrorMissingCA
			}
			return nil, fmt.Errorf("unable to load CA file: \"%s\": %s",
				*caFile, err)
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
		serverConfig.Certificates = append(serverConfig.Certificates, *tlsCert)
		if *identityCaFile != "" {
			certs, err := loadCerts(*identityCaFile)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("unable to load CA file: \"%s\": %s",
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
		srpc.RegisterServerTlsConfig(serverConfig, true)
		tricorder.RegisterMetric("/srpc/server/earliest-ca-expiration",
			&earliestCertExpiration, units.None,
			"expiration time of the CA which will expire the soonest")
	}
	return tlsCert.Leaf, nil
}
