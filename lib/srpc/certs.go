package srpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

func getEarliestCert(tlsConfig *tls.Config) *x509.Certificate {
	if tlsConfig == nil {
		return nil
	}
	var earliestCert *x509.Certificate
	for _, cert := range tlsConfig.Certificates {
		if cert.Leaf != nil && !cert.Leaf.NotAfter.IsZero() {
			if earliestCert == nil {
				earliestCert = cert.Leaf
			} else if cert.Leaf.NotAfter.Before(earliestCert.NotAfter) {
				earliestCert = cert.Leaf
			}
		}
	}
	return earliestCert
}

func getEarliestCertExpiration(tlsConfig *tls.Config) time.Time {
	var earliest time.Time
	earliestCert := getEarliestCert(tlsConfig)
	if earliestCert == nil {
		return earliest
	}
	return earliestCert.NotAfter
}

func getEarliestExpiringCertActivation(tlsConfig *tls.Config) time.Time {
	var earliest time.Time
	earliestCert := getEarliestCert(tlsConfig)
	if earliestCert == nil {
		return earliest
	}
	return earliestCert.NotBefore
}

func setupCertExpirationMetric(once sync.Once, tlsConfig **tls.Config,
	metricsDir *tricorder.DirectorySpec) {
	if tlsConfig == nil {
		return
	}
	once.Do(func() {
		metricsDir.RegisterMetric("earliest-certificate-expiration",
			func() time.Time {
				return getEarliestCertExpiration(*tlsConfig)
			},
			units.None,
			"expiration time of the certificate which will expire the soonest")
		metricsDir.RegisterMetric("earliest-expiring-certificate-activation",
			func() time.Time {
				return getEarliestExpiringCertActivation(*tlsConfig)
			},
			units.None,
			"activation time of the certificate which will expire the soonest")
	})
}

// validateTlsConfig validates a TLS configuration just prior to use.
func validateTlsConfig(config *tls.Config) error {
	if config == nil {
		return nil
	}
	if len(config.Certificates) < 1 {
		return fmt.Errorf("no certificates in TLS configuration")
	}
	now := time.Now()
	var err error
	for _, tlsCert := range config.Certificates {
		if tlsCert.Leaf == nil {
			continue
		}
		if e := validateX509Certificate(now, tlsCert.Leaf); e == nil {
			return nil
		} else if err == nil {
			err = e
		}
	}
	if err != nil {
		return fmt.Errorf("no valid certificates: %s", err)
	}
	return nil
}

// validateX509Certificate validates a certificate just prior to use.
func validateX509Certificate(now time.Time, cert *x509.Certificate) error {
	if notYet := cert.NotBefore.Sub(now); notYet > 0 {
		return fmt.Errorf("will not be valid for %s", format.Duration(notYet))
	}
	if expired := now.Sub(cert.NotAfter); expired > 0 {
		return fmt.Errorf("expired %s ago", format.Duration(expired))
	}
	return nil
}
