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
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

const (
	privateKeyFile = "key.pem"
)

func decodeKey(keyPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("error decoding PEM")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("not PEM PRIVATE KEY")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		return nil, err
	} else if key, ok := key.(*rsa.PrivateKey); !ok {
		return nil, fmt.Errorf("not an RSA key")
	} else {
		return key, nil
	}
}

// generateKey will return *rsa.PrivateKey, keyPEM, error.
func generateKey(logger log.DebugLogger) (*rsa.PrivateKey, []byte, error) {
	startTime := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, nil, err
	}
	logger.Printf("Created RSA keypair in %s\n",
		format.Duration(time.Since(startTime)))
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	buffer := &bytes.Buffer{}
	err = pem.Encode(buffer, &pem.Block{
		Bytes: keyDER,
		Type:  "PRIVATE KEY",
	})
	if err != nil {
		return nil, nil, err
	}
	return key, buffer.Bytes(), nil
}

// loadKey will return *rsa.PrivateKey, keyPEM, error.
func loadKey(filename string) (*rsa.PrivateKey, []byte, error) {
	keyPEM, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	key, err := decodeKey(keyPEM)
	if err != nil {
		return nil, nil,
			fmt.Errorf("%s: %s", filename, err)
	}
	return key, keyPEM, nil
}

// loadOrMakePrivateKey will return  *rsa.PrivateKey, keyPEM, error.
func loadOrMakePrivateKey(filename string, logger log.DebugLogger) (
	*rsa.PrivateKey, []byte, error) {
	key, keyPEM, err := loadKey(filename)
	if err == nil {
		return key, keyPEM, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}
	key, keyPEM, err = generateKey(logger)
	if err != nil {
		return nil, nil, err
	}
	err = os.WriteFile(filename, keyPEM, fsutil.PrivateFilePerms)
	if err != nil {
		return nil, nil, err
	}
	return key, keyPEM, nil
}

// makeDerPemFromPubkey returns pubkeyDER, pubkeyPEM, error.
func makeDerPemFromPubkey(pubkey *rsa.PublicKey) ([]byte, []byte, error) {
	pubkeyDER, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		return nil, nil, err
	}
	buffer := &bytes.Buffer{}
	err = pem.Encode(buffer, &pem.Block{
		Bytes: pubkeyDER,
		Type:  "PUBLIC KEY",
	})
	if err != nil {
		return nil, nil, err
	}
	return pubkeyDER, buffer.Bytes(), nil
}

func (m *Manager) loadKeys() error {
	keyFile := filepath.Join(m.StartOptions.StateDir, privateKeyFile)
	key, keyPEM, err := loadOrMakePrivateKey(keyFile, m.StartOptions.Logger)
	if err != nil {
		return err
	}
	pubkeyDER, pubkeyPEM, err := makeDerPemFromPubkey(&key.PublicKey)
	if err != nil {
		return err
	}
	m.privateKeyPEM = keyPEM
	m.publicKeyDER = pubkeyDER
	m.publicKeyPEM = pubkeyPEM
	return nil
}
