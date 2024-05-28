package srpc

import (
	"crypto/tls"
	"sync"
	"time"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

func getEarliestCertExpiration(tlsConfig *tls.Config) time.Time {
	var earliest time.Time
	if tlsConfig == nil {
		return earliest
	}
	for _, cert := range tlsConfig.Certificates {
		if cert.Leaf != nil && !cert.Leaf.NotAfter.IsZero() {
			if earliest.IsZero() {
				earliest = cert.Leaf.NotAfter
			} else if cert.Leaf.NotAfter.Before(earliest) {
				earliest = cert.Leaf.NotAfter
			}
		}
	}
	return earliest
}

func setupCertExpirationMetric(once sync.Once, tlsConfig *tls.Config,
	metricsDir *tricorder.DirectorySpec) {
	if tlsConfig == nil {
		return
	}
	once.Do(func() {
		metricsDir.RegisterMetric("earliest-certificate-expiration",
			func() time.Time {
				return getEarliestCertExpiration(tlsConfig)
			},
			units.None,
			"expiration time of the certificate which will expire the soonest")
	})
}
