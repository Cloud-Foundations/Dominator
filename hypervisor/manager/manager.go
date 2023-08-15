package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) holdLock(timeout time.Duration, writeLock bool) error {
	if timeout > time.Minute {
		return fmt.Errorf("timeout: %s exceeds one minute", timeout)
	}
	if writeLock {
		m.mutex.Lock()
		time.Sleep(timeout)
		m.mutex.Unlock()
	} else {
		m.mutex.RLock()
		time.Sleep(timeout)
		m.mutex.RUnlock()
	}
	return nil
}

func (m *Manager) setDisabledState(disable bool) error {
	m.mutex.Lock()
	doUnlock := true
	defer func() {
		if doUnlock {
			m.mutex.Unlock()
		}
	}()
	if m.disabled == disable {
		return nil
	}
	filename := filepath.Join(m.StartOptions.StateDir, "disabled")
	if disable {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			fsutil.PublicFilePerms)
		if err != nil {
			return err
		}
		file.Close()
	} else {
		if err := os.Remove(filename); err != nil {
			return err
		}
	}
	m.disabled = disable
	numFreeAddresses, err := m.computeNumFreeAddressesMap(m.addressPool)
	if err != nil {
		m.Logger.Println(err)
	}
	m.mutex.Unlock()
	doUnlock = false
	m.sendUpdate(proto.Update{
		HaveDisabled:     true,
		Disabled:         disable,
		NumFreeAddresses: numFreeAddresses,
	})
	return nil
}
