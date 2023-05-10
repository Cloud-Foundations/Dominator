package manager

import (
	"encoding/json"
	"io"
	"net"

	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type copyingReader struct {
	copyChannel chan<- byte
	r           io.Reader
}

type monitorMessageType struct {
	Data      json.RawMessage      `json:data",omitempty"`
	Event     string               `json:event",omitempty"`
	Timestamp monitorTimestampType `json:timestamp",omitempty"`
}

type monitorTimestampType struct {
	Microseconds int64 `json:microseconds",omitempty"`
	Seconds      int64 `json:seconds",omitempty"`
}

type shutdownDataType struct {
	Guest  bool   `json:guest",omitempty"`
	Reason string `json:reason",omitempty"`
}

func (r *copyingReader) Read(p []byte) (int, error) {
	nRead, err := r.r.Read(p)
	for index := 0; index < nRead; index++ {
		select {
		case r.copyChannel <- p[index]:
		default:
		}
	}
	return nRead, err
}

func (vm *vmInfoType) processMonitorResponses(monitorSock net.Conn,
	commandOutput chan<- byte) {
	reader := &copyingReader{commandOutput, monitorSock}
	decoder := json.NewDecoder(reader)
	var guestShutdown bool
	for {
		var message monitorMessageType
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				if !guestShutdown {
					vm.logger.Debugln(0, "EOF on monitor socket")
				}
				break
			}
			vm.logger.Printf("error reading monitor message: %s\n", err)
		}
		if message.Event != "SHUTDOWN" {
			continue
		}
		var shutdownData shutdownDataType
		if err := json.Unmarshal(message.Data, &shutdownData); err != nil {
			vm.logger.Printf("error unmarshaling shutdown event data: %s\n",
				err)
			continue
		}
		vm.logger.Debugf(0, "VM shutdown, guest: %v, reason: %s\n",
			shutdownData.Guest, shutdownData.Reason)
		if shutdownData.Guest && shutdownData.Reason == "guest-shutdown" {
			guestShutdown = true
		}
	}
	close(commandOutput)
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	close(vm.commandInput)
	vm.commandInput = nil
	vm.commandOutput = nil
	switch vm.State {
	case proto.StateStarting:
		select {
		case vm.stoppedNotifier <- struct{}{}:
		default:
		}
		return
	case proto.StateRunning, proto.StateDebugging:
		if guestShutdown {
			if vm.DestroyOnPowerdown && !vm.DestroyProtection {
				vm.delete()
				vm.logger.Debugln(0, "VM destroyed due to guest powerdown")
			} else {
				vm.setState(proto.StateStopped)
				vm.logger.Debugln(0, "VM stopped due to guest powerdown")
			}
		} else {
			vm.setState(proto.StateCrashed)
		}
		select {
		case vm.stoppedNotifier <- struct{}{}:
		default:
		}
		return
	case proto.StateFailedToStart:
		return
	case proto.StateStopping:
		vm.setState(proto.StateStopped)
		select {
		case vm.stoppedNotifier <- struct{}{}:
		default:
		}
	case proto.StateStopped:
		return
	case proto.StateDestroying:
		vm.delete()
		return
	case proto.StateMigrating:
		return
	case proto.StateExporting:
		return
	case proto.StateCrashed:
		vm.logger.Println("monitor socket closed on already crashed VM")
		return
	default:
		vm.logger.Println("unknown state: " + vm.State.String())
	}
}
