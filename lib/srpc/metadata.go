package srpc

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var (
	identityDocument         = "latest/dynamic/instance-identity/document"
	metadataAddress          = "http://169.254.169.254/"
	smallStackDataSource     = "datasource/SmallStack"
	smallStackOwnersLock     sync.Mutex
	_smallStackOwners        *smallStackOwnersType
	startedReadingSmallStack sync.Once
)

type smallStackOwnersType struct {
	groups []string
	users  map[string]struct{}
}

func checkSmallStack() bool {
	resp, err := http.Get(metadataAddress + smallStackDataSource)
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

func readSmallStackMetaData() {
	var vmInfo proto.VmInfo
	resp, err := http.Get(metadataAddress + identityDocument)
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
