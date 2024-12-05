package manager

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/meminfo"
	"github.com/Cloud-Foundations/Dominator/lib/rpcclientpool"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/messages"
	trimsg "github.com/Cloud-Foundations/tricorder/go/tricorder/messages"
)

const (
	boardSerialFile   = "/sys/class/dmi/id/board_serial"
	productSerialFile = "/sys/class/dmi/id/product_serial"

	uuidLength = 16
)

func getUUID(stateDir string) (string, error) {
	filename := filepath.Join(stateDir, "uuid")
	if file, err := os.Open(filename); err == nil {
		defer file.Close()
		buffer := make([]byte, uuidLength*2)
		if length, err := file.Read(buffer); err != nil {
			return "", err
		} else if length < len(buffer) {
			return "", errors.New("unable to read enough UUID data")
		} else {
			return string(buffer), nil
		}
	}
	if uuid, err := randString(uuidLength); err != nil {
		return "", err
	} else {
		os.Remove(filename)
		if file, err := os.Create(filename); err != nil {
			return "", err
		} else {
			defer file.Close()
			if _, err := fmt.Fprintln(file, uuid); err != nil {
				return "", err
			}
			return uuid, nil
		}
	}
}

func newManager(startOptions StartOptions) (*Manager, error) {
	memInfo, err := meminfo.GetMemInfo()
	if err != nil {
		return nil, err
	}
	rootCookie := make([]byte, 32)
	if _, err := rand.Read(rootCookie); err != nil {
		return nil, err
	}
	uuid, err := getUUID(startOptions.StateDir)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		StartOptions:  startOptions,
		rootCookie:    rootCookie,
		memTotalInMiB: memInfo.Total >> 20,
		notifiers:     make(map[<-chan proto.Update]chan<- proto.Update),
		numCPUs:       uint(runtime.NumCPU()),
		serialNumber:  readSystemSerial(),
		vms:           make(map[string]*vmInfoType),
		uuid:          uuid,
	}
	err = fsutil.CopyToFile(manager.GetRootCookiePath(),
		fsutil.PrivateFilePerms, bytes.NewReader(rootCookie), 0)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath.Join(startOptions.StateDir, "disabled"))
	if err == nil {
		manager.disabled = true
	}
	if err := manager.setupVolumesAndObjectCache(startOptions); err != nil {
		return nil, err
	}
	if err := manager.checkVsockets(); err != nil {
		return nil, err
	}
	if err := manager.loadKeys(); err != nil {
		return nil, err
	}
	if err := manager.loadSubnets(); err != nil {
		return nil, err
	}
	if err := manager.loadAddressPool(); err != nil {
		return nil, err
	}
	dirname := filepath.Join(manager.StateDir, "VMs")
	dir, err := os.Open(dirname)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
				return nil, errors.New(
					"error making: " + dirname + ": " + err.Error())
			}
			dir, err = os.Open(dirname)
		}
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, errors.New(
			"error reading directory: " + dirname + ": " + err.Error())
	}
	for _, ipAddr := range names {
		vmDirname := filepath.Join(dirname, ipAddr)
		filename := filepath.Join(vmDirname, "info.json")
		var vmInfo vmInfoType
		if err := json.ReadFromFile(filename, &vmInfo); err != nil {
			manager.Logger.Println(err)
			if err := os.Remove(vmDirname); err != nil {
				manager.Logger.Println(err)
			}
			continue
		}
		vmInfo.Address.Shrink()
		vmInfo.manager = manager
		vmInfo.dirname = vmDirname
		vmInfo.ipAddress = ipAddr
		vmInfo.ownerUsers = stringutil.ConvertListToMap(vmInfo.OwnerUsers,
			false)
		vmInfo.logger = prefixlogger.New(ipAddr+": ", manager.Logger)
		vmInfo.metadataChannels = make(map[chan<- string]struct{})
		manager.vms[ipAddr] = &vmInfo
		vmInfo.setupLockWatcher()
		if err := vmInfo.loadIdentityRequestorCert(); err != nil {
			vmInfo.logger.Println(
				"failed to load identity requestor certificate")
			continue
		}
		if _, err := vmInfo.startManaging(0, false, false); err != nil {
			manager.Logger.Println(err)
			if ipAddr == "0.0.0.0" {
				delete(manager.vms, ipAddr)
				vmInfo.destroy()
			}
		}
	}
	// Check address pool for used addresses with no VM, and remove.
	freeIPs := make(map[string]struct{}, len(manager.addressPool.Free))
	for _, addr := range manager.addressPool.Free {
		freeIPs[addr.IpAddress.String()] = struct{}{}
	}
	secondaryIPs := make(map[string]struct{})
	for _, vm := range manager.vms {
		for _, addr := range vm.SecondaryAddresses {
			secondaryIPs[addr.IpAddress.String()] = struct{}{}
		}
	}
	var addressesToKeep []proto.Address
	for _, addr := range manager.addressPool.Registered {
		ipAddr := addr.IpAddress.String()
		if _, ok := freeIPs[ipAddr]; ok {
			addressesToKeep = append(addressesToKeep, addr)
			continue
		}
		if _, ok := manager.vms[ipAddr]; ok {
			addressesToKeep = append(addressesToKeep, addr)
			continue
		}
		if _, ok := secondaryIPs[ipAddr]; ok {
			addressesToKeep = append(addressesToKeep, addr)
			continue
		}
		manager.Logger.Printf(
			"%s shown as used but no corresponding VM, removing\n", ipAddr)
	}
	var changedPool bool
	if len(manager.addressPool.Registered) != len(addressesToKeep) {
		manager.addressPool.Registered = addressesToKeep
		changedPool = true
	}
	// Check address pool for free addresses which are not registered and remove
	addressesToKeep = nil
	registeredIPs := make(map[string]struct{},
		len(manager.addressPool.Registered))
	for _, addr := range manager.addressPool.Registered {
		registeredIPs[addr.IpAddress.String()] = struct{}{}
	}
	for _, addr := range manager.addressPool.Free {
		ipAddr := addr.IpAddress.String()
		if _, ok := registeredIPs[ipAddr]; ok {
			addressesToKeep = append(addressesToKeep, addr)
		} else {
			manager.Logger.Printf(
				"%s shown as free but not registered, removing\n", ipAddr)
		}
	}
	if len(manager.addressPool.Free) != len(addressesToKeep) {
		manager.addressPool.Free = addressesToKeep
		changedPool = true
	}
	if changedPool {
		manager.writeAddressPoolWithLock(manager.addressPool, false)
	}
	go manager.loopCheckHealthStatus()
	lockCheckInterval := startOptions.LockCheckInterval
	if lockCheckInterval > time.Second {
		// Leveraged for dashboard, so keep it fresh.
		lockCheckInterval = time.Second
	}
	manager.lockWatcher = lockwatcher.New(&manager.mutex,
		lockwatcher.LockWatcherOptions{
			CheckInterval: lockCheckInterval,
			Logger:        startOptions.Logger,
			LogTimeout:    startOptions.LockLogTimeout,
			RFunction:     manager.updateSummaryWithMainRLock,
		})
	return manager, nil
}

func randString(length uint) (string, error) {
	buffer := make([]byte, length)
	if length, err := rand.Read(buffer); err != nil {
		return "", err
	} else if length < uuidLength {
		return "", errors.New("unable to read enough random UUID data")
	} else {
		return fmt.Sprintf("%x", buffer), nil
	}
}

func readSerialFile(filename string) string {
	if file, err := os.Open(filename); err != nil {
		return ""
	} else {
		defer file.Close()
		buffer := make([]byte, 256)
		if nRead, err := file.Read(buffer); err != nil {
			return ""
		} else if nRead < 1 {
			return ""
		} else {
			serial := strings.TrimSpace(string(buffer[:nRead]))
			// Ignore some common bogus serial numbers.
			switch serial {
			case "0123456789":
				serial = ""
			case "System Serial Number":
				serial = ""
			case "To be filled by O.E.M.":
				serial = ""
			}
			return serial
		}
	}
}

// readSystemSerial will read the product serial number and if not valid/found
// will fall back to reading the board serial number.
func readSystemSerial() string {
	if serial := readSerialFile(productSerialFile); serial != "" {
		return serial
	}
	return readSerialFile(boardSerialFile)
}

func (m *Manager) loopCheckHealthStatus() {
	cr := rpcclientpool.New("tcp", ":6910", true, "")
	for ; ; time.Sleep(time.Second * 10) {
		healthStatus := m.checkHealthStatus(cr)
		m.healthStatusMutex.Lock()
		if m.healthStatus == healthStatus {
			m.healthStatusMutex.Unlock()
			continue
		}
		m.healthStatus = healthStatus
		m.healthStatusMutex.Unlock()
		m.mutex.RLock()
		numFreeAddresses, err := m.computeNumFreeAddressesMap(m.addressPool)
		m.mutex.RUnlock()
		if err != nil {
			m.Logger.Println(err)
		}
		m.sendUpdate(proto.Update{
			NumFreeAddresses: numFreeAddresses,
		})
	}
}

func (m *Manager) checkHealthStatus(cr *rpcclientpool.ClientResource) string {
	client, err := cr.Get(nil)
	if err != nil {
		m.Logger.Printf("error getting health-agent client: %s", err)
		return "bad health-agent"
	}
	defer client.Put()
	var metric messages.Metric
	err = client.Call("MetricsServer.GetMetric", "/sys/storage/health", &metric)
	if err != nil {
		if strings.Contains(err.Error(), trimsg.ErrMetricNotFound.Error()) {
			return ""
		}
		m.Logger.Printf("error getting health-agent metrics: %s", err)
		client.Close()
		return "failed getting health metrics"
	}
	if healthStatus, ok := metric.Value.(string); !ok {
		m.Logger.Println("list metric is not string")
		return "bad health metric type"
	} else if healthStatus == "good" {
		return "healthy"
	} else {
		return healthStatus
	}
}
