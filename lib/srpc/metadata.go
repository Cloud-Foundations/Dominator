package srpc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var (
	smallStackOwnersLock     sync.Mutex
	_smallStackOwners        *smallStackOwnersType
	startedReadingSmallStack sync.Once
)

type smallStackOwnersType struct {
	groups []string
	users  map[string]struct{}
}

func checkSmallStack() bool {
	resp, err := http.Get(constants.MetadataUrl +
		constants.SmallStackDataSource)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()
	buffer := make([]byte, 10)
	if length, _ := resp.Body.Read(buffer); length >= 4 {
		if string(buffer[:4]) == "true" {
			return true
		}
	}
	return false
}

func getSmallStackOwners() *smallStackOwnersType {
	smallStackOwnersLock.Lock()
	defer smallStackOwnersLock.Unlock()
	return _smallStackOwners
}

func loadCertificatesFromMetadata(timeout time.Duration, errorIfMissing bool,
	errorIfExpired bool) (
	*tls.Certificate, error) {
	certPEM, err := readMetadataFile(constants.MetadataIdentityCert, timeout)
	if err != nil {
		if errorIfMissing {
			return nil, err
		}
		return nil, nil
	}
	keyPEM, err := readMetadataFile(constants.MetadataIdentityKey, timeout)
	if err != nil {
		if errorIfMissing {
			return nil, err
		}
		return nil, nil
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}
	if errorIfExpired {
		now := time.Now()
		if notYet := x509Cert.NotBefore.Sub(now); notYet > 0 {
			return nil, fmt.Errorf("cert will not be valid for %s",
				format.Duration(notYet))
		}
		if expired := now.Sub(x509Cert.NotAfter); expired > 0 {
			return nil, fmt.Errorf("cert expired %s ago",
				format.Duration(expired))
		}
	}
	tlsCert.Leaf = x509Cert
	return &tlsCert, nil
}

func readMetadataFile(filename string, timeout time.Duration) ([]byte, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(constants.MetadataUrl + filename)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readSmallStackMetaData() {
	var vmInfo proto.VmInfo
	resp, err := http.Get(constants.MetadataUrl + constants.MetadataIdentityDoc)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&vmInfo); err != nil {
		return
	}
	smallStackOwners := &smallStackOwnersType{
		groups: vmInfo.OwnerGroups,
		users:  stringutil.ConvertListToMap(vmInfo.OwnerUsers, false),
	}
	logger.Debugf(1, "VM OwnerUsers: %v, OwnerGroups: %v\n",
		vmInfo.OwnerUsers, vmInfo.OwnerGroups)
	smallStackOwnersLock.Lock()
	defer smallStackOwnersLock.Unlock()
	_smallStackOwners = smallStackOwners
}

func readSmallStackMetaDataLoop() {
	if !checkSmallStack() {
		return
	}
	logger.Debugln(0,
		"Running on SmallStack: will grant method access to VM owners")
	for ; true; time.Sleep(10 * time.Second) {
		readSmallStackMetaData()
	}
}

func startReadingSmallStackMetaData() {
	if !*srpcTrustVmOwners {
		return
	}
	startedReadingSmallStack.Do(func() { go readSmallStackMetaDataLoop() })
}
