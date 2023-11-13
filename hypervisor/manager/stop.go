package manager

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type flusher interface {
	Flush() error
}

func (m *Manager) powerOff(stopVMs bool) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if stopVMs {
		m.shutdownVMs()
	}
	for _, vm := range m.vms {
		if vm.State != proto.StateStopped {
			return fmt.Errorf("%s is not shut down", vm.Address.IpAddress)
		}
	}
	cmd := exec.Command("poweroff")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func (m *Manager) shutdownVMs() {
	m.shuttingDown = true
	var waitGroup sync.WaitGroup
	var failCount uint
	var failMutex sync.Mutex
	for _, vm := range m.vms {
		waitGroup.Add(1)
		go func(vm *vmInfoType) {
			defer waitGroup.Done()
			if !vm.shutdown() {
				failMutex.Lock()
				failCount++
				failMutex.Unlock()
			}
		}(vm)
	}
	waitGroup.Wait()
	if failCount > 1 {
		m.Logger.Printf("stopping but failed to cleanly shut down %d VMs\n",
			failCount)
	} else if failCount > 0 {
		m.Logger.Println("stopping but failed to cleanly shut down 1 VM")
	} else {
		m.Logger.Println("stopping cleanly after shutting down VMs")
	}
	if flusher, ok := m.Logger.(flusher); ok {
		flusher.Flush()
	}
}

func (m *Manager) shutdownVMsAndExit() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	m.shutdownVMs()
	os.Exit(0)
}

// Returns false if the VM failed to shut down cleanly, else true.
func (vm *vmInfoType) shutdown() bool {
	vm.mutex.RLock()
	switch vm.State {
	case proto.StateStarting, proto.StateRunning:
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.commandInput <- "system_powerdown"
		vm.mutex.RUnlock()
		timer := time.NewTimer(time.Minute)
		select {
		case <-stoppedNotifier:
			if !timer.Stop() {
				<-timer.C
			}
			vm.logger.Println("shut down cleanly for system shutdown")
		case <-timer.C:
			vm.logger.Println("shutdown timed out: killing VM")
			vm.commandInput <- "quit"
			return false
		}
	default:
		vm.mutex.RUnlock()
	}
	return true
}
