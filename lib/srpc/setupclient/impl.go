package setupclient

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const dateTime = time.DateTime + " MST"

func getEarliestCertExpiration(certs []tls.Certificate) *x509.Certificate {
	var earliest *x509.Certificate
	for _, cert := range certs {
		if cert.Leaf != nil && !cert.Leaf.NotAfter.IsZero() {
			if earliest == nil {
				earliest = cert.Leaf
			} else if cert.Leaf.NotAfter.Before(earliest.NotAfter) {
				earliest = cert.Leaf
			}
		}
	}
	return earliest
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

func loadLoop(logger log.DebugLogger, cert *x509.Certificate) {
	for {
		sleepInterval := getSleepInterval(cert)
		logger.Printf("Certificate refetch at: %s (%s)\n",
			time.Now().Add(sleepInterval).Format(dateTime),
			format.Duration(sleepInterval))
		time.Sleep(sleepInterval)
		if c, err := setupTlsOnce(logger, true); err != nil {
			logger.Println(err)
		} else {
			cert = c
		}
	}
}

func setupTls(ignoreMissingCerts bool) error {
	logger := nulllogger.New()
	cert, err := setupTlsOnce(logger, !ignoreMissingCerts)
	if err != nil {
		return err
	}
	if cert != nil {
		go loadLoop(logger, cert)
	}
	return nil
}

func setupTlsOnce(logger log.DebugLogger, failIfExpired bool) (
	*x509.Certificate, error) {
	certs, err := loadCerts()
	if err != nil {
		return nil, err
	}
	if certs == nil {
		if failIfExpired {
			return nil, srpc.ErrorMissingCertificate
		}
		return nil, nil
	}
	// Check expiration of earliest certificate.
	earliest := getEarliestCertExpiration(certs)
	if earliest != nil {
		now := time.Now()
		if notYet := earliest.NotBefore.Sub(now); notYet > 0 {
			msg := "certificate will not be valid for " +
				format.Duration(notYet)
			if failIfExpired {
				logger.Println(msg)
				return nil, srpc.ErrorMissingCertificate
			}
			logger.Println(msg)
		} else if expired := now.Sub(earliest.NotAfter); expired > 0 {
			msg := "certificate expired " + format.Duration(expired) + " ago"
			if failIfExpired {
				logger.Println(msg)
				return nil, srpc.ErrorMissingCertificate
			}
			logger.Println(msg)
		} else {
			logger.Printf("Certificate expires at: %s (%s)\n",
				earliest.NotAfter.Local().Format(dateTime),
				format.Duration(time.Until(earliest.NotAfter)))
		}
	}
	// Setup client.
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = certs
	srpc.RegisterClientTlsConfig(clientConfig)
	return earliest, nil
}
