package manager

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	"golang.org/x/crypto/ssh"
)

const (
	certificateRequestPathFormat = "/certgen/%s"
	identityRequestorCertFile    = "identityRequestor.cert"
	refreshRoleRequestPath       = "/v1/refreshRoleRequestingCert"
)

type pubkeysType struct {
	ssh     []byte
	x509PEM []byte
}

func loadCertAndSetDeadline(refresher *backoffdelay.Refresher,
	timer *time.Timer, filename string, certType string, logger log.Logger) {
	certPEM, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	cert, err := decodeCert(certPEM)
	if err != nil {
		logger.Println(err)
		return
	}
	refresher.SetDeadline(cert.NotAfter)
	username, _ := x509util.GetUsername(cert)
	logger.Printf(
		"loaded %s certificate for: \"%s\", expires at: %s (in %s)\n",
		certType, username, cert.NotAfter.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(cert.NotAfter)))
	refresher.ResetTimer(timer)
}

func removeFileOrLog(filename string, logger log.Logger) {
	if err := os.Remove(filename); err != nil {
		if !os.IsNotExist(err) {
			logger.Println(err)
		}
	}
}

func requestCertificate(httpClient *http.Client, pubkey []byte,
	identityProvider, username, certType string,
	logger log.DebugLogger) ([]byte, error) {
	baseUrl, err := url.Parse(identityProvider)
	certificateRequestPath := fmt.Sprintf(certificateRequestPathFormat,
		username)
	requestUrl := url.URL{
		Scheme: baseUrl.Scheme,
		Host:   baseUrl.Host,
		Path:   certificateRequestPath,
	}
	switch certType {
	case "SSH":
		requestUrl.RawQuery = "type=ssh"
	case "X.509":
		requestUrl.RawQuery = "type=x509"
	}
	buffer := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(buffer)
	fileWriter, err := bodyWriter.CreateFormFile("pubkeyfile",
		"somefilename.pub")
	if err != nil {
		return nil, err
	}
	if _, err := fileWriter.Write(pubkey); err != nil {
		return nil, err
	}
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()
	logger.Debugf(0, "requesting %s identity certificate from: %s\n",
		certType, requestUrl.String())
	req, err := http.NewRequest("POST", requestUrl.String(), buffer)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func stopTimer(timer *time.Timer) {
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
}

func (m *Manager) replaceVmCredentials(
	request proto.ReplaceVmCredentialsRequest,
	authInfo *srpc.AuthInformation) error {
	tlsCert, identityName, err := validateIdentityKeyPair(
		request.IdentityCertificate, request.IdentityKey, authInfo.Username)
	if err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	err = writeKeyPair(request.IdentityCertificate, request.IdentityKey,
		filepath.Join(vm.dirname, IdentityCertFile),
		filepath.Join(vm.dirname, IdentityKeyFile))
	if err != nil {
		return err
	}
	if !vm.IdentityExpires.Equal(tlsCert.Leaf.NotAfter) ||
		vm.IdentityName != identityName {
		vm.IdentityExpires = tlsCert.Leaf.NotAfter
		vm.IdentityName = identityName
		vm.writeAndSendInfo()
	}
	return nil
}

func (m *Manager) replaceVmIdentity(
	request proto.ReplaceVmIdentityRequest,
	authInfo *srpc.AuthInformation) error {
	if len(request.IdentityRequestorCertificate) < 1 {
		// Erase identity.
		vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
		if err != nil {
			return err
		}
		defer vm.mutex.Unlock()
		if vm.identityProviderNotifier != nil {
			close(vm.identityProviderNotifier)
			vm.identityProviderNotifier = nil
		}
		vm.identityProviderTransport = nil
		vm.IdentityExpires = time.Time{}
		vm.IdentityName = ""
		removeFileOrLog(filepath.Join(vm.dirname, IdentityCertFile), vm.logger)
		removeFileOrLog(filepath.Join(vm.dirname, IdentityKeyFile), vm.logger)
		removeFileOrLog(filepath.Join(vm.dirname, IdentitySshCertFile),
			vm.logger)
		removeFileOrLog(filepath.Join(vm.dirname, identityRequestorCertFile),
			vm.logger)
		vm.writeAndSendInfo()
		return nil
	}
	if m.StartOptions.IdentityProvider == "" {
		return fmt.Errorf("no Identity Provider")
	}
	tlsCert, username, err := validateIdentityKeyPair(
		request.IdentityRequestorCertificate,
		m.privateKeyPEM, authInfo.Username)
	if err != nil {
		return err
	}
	expiresAt := tlsCert.Leaf.NotAfter
	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{*tlsCert},
			InsecureSkipVerify: true,
		},
	}
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.logger.Printf(
		"received identity requestor certificate for: \"%s\", expires at: %s (in %s)\n",
		username, expiresAt.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(expiresAt)))
	reader := bytes.NewReader(request.IdentityRequestorCertificate)
	err = fsutil.CopyToFile(
		filepath.Join(vm.dirname, identityRequestorCertFile),
		fsutil.PublicFilePerms, reader, 0)
	if err != nil {
		return err
	}
	vm.identityProviderTransport = &transport
	if vm.identityProviderNotifier == nil {
		notifier := make(chan time.Time, 1)
		vm.identityProviderNotifier = notifier
		go vm.refreshCredentialsLoop(notifier)
	}
	vm.identityProviderNotifier <- expiresAt
	return nil
}

func (vm *vmInfoType) loadIdentityRequestorCert() error {
	certPEM, err := os.ReadFile(
		filepath.Join(vm.dirname, identityRequestorCertFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if vm.manager.StartOptions.IdentityProvider == "" {
		return fmt.Errorf("no Identity Provider")
	}
	tlsCert, username, err := parseKeyPair(certPEM, vm.manager.privateKeyPEM)
	if err != nil {
		return err
	}
	expiresAt := tlsCert.Leaf.NotAfter
	vm.logger.Printf(
		"loaded identity requestor certificate for: \"%s\", expires at: %s (in %s)\n",
		username, expiresAt.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(expiresAt)))
	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{*tlsCert},
			InsecureSkipVerify: true,
		},
	}
	vm.identityProviderTransport = &transport
	notifier := make(chan time.Time, 1)
	vm.identityProviderNotifier = notifier
	go vm.refreshCredentialsLoop(notifier)
	vm.identityProviderNotifier <- expiresAt
	return nil
}

func (vm *vmInfoType) refreshCredentials(pubkeys pubkeysType) (
	time.Time, error) {
	vm.mutex.RLock()
	transport := vm.identityProviderTransport
	vm.mutex.RUnlock()
	username, err := x509util.GetUsername(
		transport.TLSClientConfig.Certificates[0].Leaf)
	if err != nil {
		return time.Time{}, err
	}
	httpClient := &http.Client{Transport: transport}
	// Request, parse and write X.509 certificate.
	x509CertPEM, err := requestCertificate(httpClient, pubkeys.x509PEM,
		vm.manager.StartOptions.IdentityProvider, username, "X.509",
		vm.logger)
	cert, err := decodeCert(x509CertPEM)
	if err != nil {
		return time.Time{}, err
	}
	reader := bytes.NewReader(x509CertPEM)
	err = fsutil.CopyToFile(
		filepath.Join(vm.dirname, IdentityCertFile),
		fsutil.PublicFilePerms, reader, 0)
	if err != nil {
		return time.Time{}, err
	}
	// Request and write SSH certificate.
	sshCertEnc, err := requestCertificate(httpClient, pubkeys.ssh,
		vm.manager.StartOptions.IdentityProvider, username, "SSH",
		vm.logger)
	reader = bytes.NewReader(sshCertEnc)
	err = fsutil.CopyToFile(
		filepath.Join(vm.dirname, IdentitySshCertFile),
		fsutil.PublicFilePerms, reader, 0)
	if err != nil {
		return time.Time{}, err
	}
	// Log and record.
	expiresIn := time.Until(cert.NotAfter)
	vm.logger.Printf(
		"new identity certificate for: \"%s\", expires at: %s (in %s)\n",
		username, cert.NotAfter.Format(format.TimeFormatSeconds),
		format.Duration(expiresIn))
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	vm.IdentityExpires = cert.NotAfter
	vm.IdentityName = username
	vm.writeAndSendInfo()
	return cert.NotAfter, nil
}

func (vm *vmInfoType) refreshCredentialsLoop(notifier <-chan time.Time) {
	key, _, err := loadOrMakePrivateKey(
		filepath.Join(vm.dirname, IdentityKeyFile),
		vm.logger)
	if err != nil {
		vm.logger.Println(err)
		return
	}
	_, pubkeyPEM, err := makeDerPemFromPubkey(&key.PublicKey)
	if err != nil {
		vm.logger.Printf("error making X.509 public key: %s\n", err)
		return
	}
	sshPub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		vm.logger.Printf("error making SSH public key: %s\n", err)
		return
	}
	pubkeys := pubkeysType{
		ssh:     ssh.MarshalAuthorizedKey(sshPub),
		x509PEM: pubkeyPEM,
	}
	requestorRefresher := backoffdelay.NewRefresher(<-notifier, 0, 0)
	requestorTimer := time.NewTimer(0)
	requestorRefresher.ResetTimer(requestorTimer)
	roleRefresher := backoffdelay.NewRefresher(time.Time{}, 0, 0)
	roleTimer := time.NewTimer(0)
	loadCertAndSetDeadline(roleRefresher, roleTimer,
		filepath.Join(vm.dirname, IdentityCertFile),
		"identity", vm.logger)
	for {
		select {
		case requestorExpiresAt, ok := <-notifier:
			stopTimer(requestorTimer)
			if !ok {
				stopTimer(roleTimer)
				return
			}
			requestorRefresher.SetDeadline(requestorExpiresAt)
			requestorRefresher.ResetTimer(requestorTimer)
		case <-requestorTimer.C:
			if expiresAt, err := vm.refreshRequestor(); err != nil {
				vm.logger.Println(err)
			} else {
				requestorRefresher.SetDeadline(expiresAt)
			}
			requestorRefresher.ResetTimer(requestorTimer)
		case <-roleTimer.C:
			if expiresAt, err := vm.refreshCredentials(pubkeys); err != nil {
				vm.logger.Println(err)
			} else {
				roleRefresher.SetDeadline(expiresAt)
			}
			roleRefresher.ResetTimer(roleTimer)
		}
	}
}

func (vm *vmInfoType) refreshRequestor() (time.Time, error) {
	vm.mutex.RLock()
	transport := vm.identityProviderTransport
	vm.mutex.RUnlock()
	username, err := x509util.GetUsername(
		transport.TLSClientConfig.Certificates[0].Leaf)
	if err != nil {
		return time.Time{}, err
	}
	httpClient := http.Client{Transport: transport}
	baseUrl, err := url.Parse(vm.manager.StartOptions.IdentityProvider)
	if err != nil {
		return time.Time{}, err
	}
	requestUrl := url.URL{
		Scheme: baseUrl.Scheme,
		Host:   baseUrl.Host,
		Path:   refreshRoleRequestPath,
	}
	vm.logger.Debugf(0,
		"requesting identity requestor certificate for: %s from: %s\n",
		username, requestUrl.String())
	resp, err := httpClient.PostForm(
		requestUrl.String(),
		url.Values{
			"pubkey": []string{
				base64.RawURLEncoding.EncodeToString(vm.manager.publicKeyDER)},
		})
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Time{},
			fmt.Errorf("error getting identity requesting certificate: %s",
				resp.Status)
	}
	certPEM, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{},
			fmt.Errorf("error reading identity requesting certificate: %s", err)
	}
	tlsCert, username, err := parseKeyPair(certPEM, vm.manager.privateKeyPEM)
	if err != nil {
		return time.Time{}, err
	}
	expiresAt := tlsCert.Leaf.NotAfter
	vm.logger.Printf(
		"refreshed identity requestor certificate for: \"%s\", expires at: %s (in %s)\n",
		username, expiresAt.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(expiresAt)))
	transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{*tlsCert},
			InsecureSkipVerify: true,
		},
	}
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	reader := bytes.NewReader(certPEM)
	err = fsutil.CopyToFile(
		filepath.Join(vm.dirname, identityRequestorCertFile),
		fsutil.PublicFilePerms, reader, 0)
	if err != nil {
		return time.Time{}, err
	}
	vm.identityProviderTransport = transport
	return expiresAt, nil
}
