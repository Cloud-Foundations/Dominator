package manager

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

const (
	privateKeyFile = "key.pem"
)

func (m *Manager) loadKeys() error {
	keyFile := filepath.Join(m.StartOptions.StateDir, privateKeyFile)
	var key *rsa.PrivateKey
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		startTime := time.Now()
		key, err = rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			return err
		}
		m.StartOptions.Logger.Printf("Created RSA keypair in %s\n",
			format.Duration(time.Since(startTime)))
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return err
		}
		buffer := &bytes.Buffer{}
		err = pem.Encode(buffer, &pem.Block{
			Bytes: keyDER,
			Type:  "PRIVATE KEY",
		})
		if err != nil {
			return err
		}
		keyPEM = buffer.Bytes()
		err = os.WriteFile(keyFile, keyPEM, fsutil.PrivateFilePerms)
		if err != nil {
			return err
		}
	} else {
		block, _ := pem.Decode(keyPEM)
		if block == nil {
			return fmt.Errorf("error decoding PEM private key in file: %s",
				keyFile)
		}
		if block.Type != "PRIVATE KEY" {
			return fmt.Errorf("not PEM PRIVATE KEY")
		}
		if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
			return err
		} else if k, ok := k.(*rsa.PrivateKey); !ok {
			return fmt.Errorf("not an RSA key")
		} else {
			key = k
		}
	}
	pubkeyDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	buffer := &bytes.Buffer{}
	err = pem.Encode(buffer, &pem.Block{
		Bytes: pubkeyDER,
		Type:  "PUBLIC KEY",
	})
	if err != nil {
		return err
	}
	pubkeyPEM := buffer.Bytes()
	m.privateKeyPEM = keyPEM
	m.publicKeyPEM = pubkeyPEM
	return nil
}
