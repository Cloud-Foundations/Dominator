package setupclient

import (
	"crypto/tls"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const dateTime = time.DateTime + " MST"

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

func loadLoop(params Params) {
	refresher := newRefresher()
	for {
		sleepInterval := refresher.WaitInterval()
		params.Logger.Debugf(0, "Certificate refetch at: %s (%s)\n",
			time.Now().Add(sleepInterval).Format(dateTime),
			format.Duration(sleepInterval))
		time.Sleep(sleepInterval)
		if err := setupTlsOnce(params); err != nil {
			params.Logger.Println(err)
		} else {
			refresher.SetDeadline(srpc.GetEarliestClientCertExpiration())
		}
	}
}

func newRefresher() *backoffdelay.Refresher {
	expiration := srpc.GetEarliestClientCertExpiration()
	if !expiration.IsZero() {
		return backoffdelay.NewRefresher(expiration, time.Minute, 0)
	}
	return backoffdelay.NewRefresher(time.Now().Add(time.Hour), time.Minute, 0)
}

func setupTls(params Params) error {
	if params.Logger == nil {
		params.Logger = nulllogger.New()
	}
	if err := setupTlsOnce(params); err != nil {
		return err
	}
	go loadLoop(params)
	return nil
}

func setupTlsOnce(params Params) error {
	certs, err := loadCerts()
	if err != nil {
		return err
	}
	if certs == nil {
		if params.IgnoreMissingCerts {
			return nil
		}
		return srpc.ErrorMissingCertificate
	}
	// Setup client.
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = certs
	srpc.RegisterClientTlsConfig(clientConfig)
	return nil
}
