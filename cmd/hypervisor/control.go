package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

var shutdownVMsOnNextStop bool

type flusher interface {
	Flush() error
}

func acceptControlConnections(m *manager.Manager, listener net.Listener,
	logger log.DebugLogger) {
	for {
		if conn, err := listener.Accept(); err != nil {
			logger.Println(err)
		} else if err := processControlConnection(conn, m, logger); err != nil {
			logger.Println(err)
		}
	}
}

func connectToControl() net.Conn {
	sockAddr := filepath.Join(*stateDir, "control")
	if conn, err := net.Dial("unix", sockAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to: %s: %s\n", sockAddr, err)
		os.Exit(1)
		return nil
	} else {
		return conn
	}
}

func listenForControl(m *manager.Manager, logger log.DebugLogger) error {
	sockAddr := filepath.Join(*stateDir, "control")
	os.Remove(sockAddr)
	if listener, err := net.Listen("unix", sockAddr); err != nil {
		return err
	} else {
		if err := os.Chmod(sockAddr, fsutil.PrivateFilePerms); err != nil {
			return err
		}
		go acceptControlConnections(m, listener, logger)
		return nil
	}
}

func processControlConnection(conn net.Conn, m *manager.Manager,
	logger log.DebugLogger) error {
	defer conn.Close()
	buffer := make([]byte, 256)
	if nRead, err := conn.Read(buffer); err != nil {
		return fmt.Errorf("error reading request: %s", err)
	} else if nRead < 1 {
		return fmt.Errorf("read short request: %s", err)
	} else {
		request := string(buffer[:nRead])
		if request[nRead-1] != '\n' {
			return fmt.Errorf("request not null-terminated: %s", request)
		}
		request = request[:nRead-1]
		switch request {
		case "stop":
			if _, err := fmt.Fprintln(conn, "ok"); err != nil {
				return err
			}
			os.Remove(m.GetRootCookiePath())
			if shutdownVMsOnNextStop {
				logger.Println("shutting down VMs and stopping")
				m.ShutdownVMsAndExit()
			} else {
				logger.Println("stopping without shutting down VMs")
				if flusher, ok := logger.(flusher); ok {
					flusher.Flush()
				}
				os.Exit(0)
			}
		case "stop-vms-on-next-stop":
			if _, err := fmt.Fprintln(conn, "ok"); err != nil {
				return err
			}
			shutdownVMsOnNextStop = true
			logger.Println("will shut down VMs on next stop")
		default:
			if _, err := fmt.Fprintln(conn, "bad request"); err != nil {
				return err
			}
		}
	}
	return nil
}

func sendRequest(conn net.Conn, request string) error {
	if _, err := fmt.Fprintln(conn, request); err != nil {
		return fmt.Errorf("error writing request: %s\n", err)
	}
	buffer := make([]byte, 256)
	if nRead, err := conn.Read(buffer); err != nil {
		return fmt.Errorf("error reading response: %s\n", err)
	} else if nRead < 1 {
		return fmt.Errorf("read short response: %s\n", err)
	} else {
		response := string(buffer[:nRead])
		if response[nRead-1] != '\n' {
			return fmt.Errorf("response not null-terminated: %s\n", response)
		}
		response = response[:nRead-1]
		if response != "ok" {
			return fmt.Errorf("bad response: %s\n", response)
		} else {
			conn.Read(buffer) // Wait for EOF.
			conn.Close()
			return nil
		}
	}
}

func stopSubcommand(args []string, logger log.DebugLogger) error {
	return sendRequest(connectToControl(), "stop")
}

func stopVmsOnNextStopSubcommand(args []string, logger log.DebugLogger) error {
	return sendRequest(connectToControl(), "stop-vms-on-next-stop")
}
