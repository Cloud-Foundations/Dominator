package setupserver

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var (
	caFile = flag.String("CAfile", "/etc/ssl/CA.pem",
		"Name of file containing the root of trust for identity and methods")
	certFile = flag.String("certFile",
		path.Join("/etc/ssl", getDirname(), "cert.pem"),
		"Name of file containing the SSL certificate")
	identityCaFile = flag.String("identityCAfile", "/etc/ssl/IdentityCA.pem",
		"Name of file containing the root of trust for identity only")
	keyFile = flag.String("keyFile",
		path.Join("/etc/ssl", getDirname(), "key.pem"),
		"Name of file containing the SSL key")
)

func getDirname() string {
	return path.Base(os.Args[0])
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
	// Load certificate and key.
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		return nil, fmt.Errorf("unable to load keypair: %s", err)
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}
	cert.Leaf = x509Cert
	params.Logger.Debugf(0, "Loaded certifcate and key from: %s and %s\n",
		*certFile, *keyFile)
	now := time.Now()
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
	if !params.ClientOnly {
		caData, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load CA file: \"%s\": %s",
				*caFile, err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("unable to parse CA file")
		}
		serverConfig := new(tls.Config)
		serverConfig.ClientAuth = tls.RequireAndVerifyClientCert
		serverConfig.MinVersion = tls.VersionTLS12
		serverConfig.ClientCAs = caCertPool
		serverConfig.Certificates = append(serverConfig.Certificates, cert)
		if *identityCaFile != "" {
			identityCaData, err := ioutil.ReadFile(*identityCaFile)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("unable to load CA file: \"%s\": %s",
						*caFile, err)
				}
			} else {
				srpc.RegisterFullAuthCA(caCertPool)
				caCertPool := x509.NewCertPool()
				if !caCertPool.AppendCertsFromPEM(caData) {
					return nil, fmt.Errorf("unable to parse CA file")
				}
				if !caCertPool.AppendCertsFromPEM(identityCaData) {
					return nil, fmt.Errorf("unable to parse identity CA file")
				}
				serverConfig.ClientCAs = caCertPool
			}
		}
		srpc.RegisterServerTlsConfig(serverConfig, true)
	}
	// Setup client.
	clientConfig := new(tls.Config)
	clientConfig.InsecureSkipVerify = true
	clientConfig.MinVersion = tls.VersionTLS12
	clientConfig.Certificates = append(clientConfig.Certificates, cert)
	srpc.RegisterClientTlsConfig(clientConfig)
	return cert.Leaf, nil
}
