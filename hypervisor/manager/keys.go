package manager

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
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
	"golang.org/x/crypto/ssh"
)

const (
	algorithmEd25519 = algorithmType(iota)
	algorithmRsa

	privateKeyFile = "key.pem"
)

type algorithmType uint

func decodeKey(keyPEM []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("error decoding PEM")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("not PEM PRIVATE KEY")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		return nil, err
	} else if rsaKey, ok := key.(*rsa.PrivateKey); ok {
		return rsaKey, nil
	} else if ed25519Key, ok := key.(ed25519.PrivateKey); ok {
		return ed25519Key, nil
	} else {
		return nil, fmt.Errorf("not an RSA or Ed25519 key, type: %T", key)
	}
}

// generateKey will return crypto.Signer, keyPEM, error.
func generateKey(algorithm algorithmType, logger log.DebugLogger) (
	crypto.Signer, []byte, error) {
	var key crypto.Signer
	var err error
	startTime := time.Now()
	switch algorithm {
	case algorithmEd25519:
		_, key, err = ed25519.GenerateKey(rand.Reader)
	case algorithmRsa:
		key, err = rsa.GenerateKey(rand.Reader, 3072)
	default:
		return nil, nil,
			fmt.Errorf("cannot generate key of type: %s", algorithm)

	}
	if err != nil {
		return nil, nil, err
	}
	logger.Printf("created %s keypair in %s\n",
		algorithm, format.Duration(time.Since(startTime)))
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

// loadKey will return crypto.Signer, keyPEM, error.
func loadKey(filename string) (crypto.Signer, []byte, error) {
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

// loadOrMakePrivateKey will return  crypto.Signer, keyPEM, error.
func loadOrMakePrivateKey(algorithm algorithmType, filename, sshFilename string,
	logger log.DebugLogger) (
	crypto.Signer, []byte, error) {
	key, keyPEM, err := loadKey(filename)
	if err == nil {
		return key, keyPEM, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}
	key, keyPEM, err = generateKey(algorithm, logger)
	if err != nil {
		return nil, nil, err
	}
	err = os.WriteFile(filename, keyPEM, fsutil.PrivateFilePerms)
	if err != nil {
		return nil, nil, err
	}
	if algorithm == algorithmEd25519 {
		block, err := ssh.MarshalPrivateKey(key, "")
		if err != nil {
			return nil, nil, err
		}
		err = os.WriteFile(sshFilename, pem.EncodeToMemory(block),
			fsutil.PrivateFilePerms)
		if err != nil {
			return nil, nil, err
		}
	}
	return key, keyPEM, nil
}

// makeDerPemFromPubkey returns pubkeyDER, pubkeyPEM, error.
func makeDerPemFromPubkey(pubkey crypto.PublicKey) ([]byte, []byte, error) {
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

func (algorithm algorithmType) String() string {
	switch algorithm {
	case algorithmEd25519:
		return "Ed25519"
	case algorithmRsa:
		return "RSA"
	default:
		return fmt.Sprintf("unknown algorithm type: %d", algorithm)
	}
}

func (m *Manager) loadKeys() error {
	keyFile := filepath.Join(m.StartOptions.StateDir, privateKeyFile)
	key, keyPEM, err := loadOrMakePrivateKey(algorithmRsa, keyFile, "",
		m.StartOptions.Logger)
	if err != nil {
		return err
	}
	pubkeyDER, pubkeyPEM, err := makeDerPemFromPubkey(key.Public())
	if err != nil {
		return err
	}
	m.privateKeyPEM = keyPEM
	m.publicKeyDER = pubkeyDER
	m.publicKeyPEM = pubkeyPEM
	return nil
}
