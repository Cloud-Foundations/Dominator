package srpc

import (
	"crypto/tls"
	"time"
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
