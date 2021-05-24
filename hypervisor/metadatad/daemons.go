// +build go1.10

package metadatad

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type statusType struct {
	namespaceFd int
	threadId    int
	err         error
}

func httpServe(listener net.Listener, handler http.Handler,
	idleTimeout time.Duration) error {
	httpServer := &http.Server{Handler: handler, IdleTimeout: idleTimeout}
	return httpServer.Serve(listener)
}

func (s *server) startServer() error {
	cmd := exec.Command("ebtables", "-t", "nat", "-F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error running ebtables: %s: %s", err, string(output))
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, bridge := range s.bridges {
		if err := s.startServerOnBridge(bridge); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) startServerOnBridge(bridge net.Interface) error {
	logger := prefixlogger.New(bridge.Name+": ", s.logger)
	startChannel := make(chan struct{})
	statusChannel := make(chan statusType, 1)
	go s.createNamespace(startChannel, statusChannel, logger)
	status := <-statusChannel
	if status.err != nil {
		return status.err
	}
	if err := createInterface(bridge, status.threadId, logger); err != nil {
		return err
	}
	startChannel <- struct{}{}
	status = <-statusChannel
	if status.err != nil {
		return status.err
	}
	subnetChannel := s.manager.MakeSubnetChannel()
	go s.addSubnets(bridge, status.namespaceFd, subnetChannel, logger)
	return nil
}

func (s *server) addSubnets(bridge net.Interface, namespaceFd int,
	subnetChannel <-chan proto.Subnet, logger log.DebugLogger) {
	logger.Debugf(0, "waiting for subnet updates in namespaceFD=%d\n",
		namespaceFd)
	if err := wsyscall.SetNetNamespace(namespaceFd); err != nil {
		logger.Println(err)
		return
	}
	for subnet := range subnetChannel {
		addRouteForBridge(bridge, subnet, logger)
	}
}

func addRouteForBridge(bridge net.Interface, subnet proto.Subnet,
	logger log.DebugLogger) {
	if subnet.DisableMetadata {
		logger.Debugf(0, "metadata service disabled for subnet: %s\n",
			subnet.Id)
		return
	}
	subnetMask := net.IPMask(subnet.IpMask)
	subnetAddr := subnet.IpGateway.Mask(subnetMask)
	addr := subnetAddr.String()
	mask := fmt.Sprintf("%d.%d.%d.%d",
		subnetMask[0], subnetMask[1], subnetMask[2], subnetMask[3])
	cmd := exec.Command("route", "add", "-net", addr, "netmask", mask, "eth0")
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Printf("error adding route: for subnet: %s: %s/%s: %s: %s",
			subnet.Id, addr, mask, err, string(output))
	} else {
		logger.Debugf(0, "added route for subnet: %s: %s/%s\n",
			subnet.Id, addr, mask)
	}
}

func (s *server) createNamespace(startChannel <-chan struct{},
	statusChannel chan<- statusType, logger log.DebugLogger) {
	namespaceFd, threadId, err := wsyscall.UnshareNetNamespace()
	if err != nil {
		statusChannel <- statusType{err: err}
		return
	}
	statusChannel <- statusType{namespaceFd: namespaceFd, threadId: threadId}
	<-startChannel
	cmd := exec.Command("ifconfig", "eth0", "169.254.169.254", "netmask",
		"255.255.255.255", "up")
	if err := cmd.Run(); err != nil {
		statusChannel <- statusType{err: err}
		return
	}
	hypervisorListener, err := net.Listen("tcp",
		fmt.Sprintf("169.254.169.254:%d", s.hypervisorPortNum))
	if err != nil {
		statusChannel <- statusType{err: err}
		return
	}
	metadataListener, err := net.Listen("tcp", "169.254.169.254:80")
	if err != nil {
		statusChannel <- statusType{err: err}
		return
	}
	statusChannel <- statusType{namespaceFd: namespaceFd, threadId: threadId}
	logger.Printf("starting metadata server in thread: %d\n", threadId)
	go httpServe(hypervisorListener, nil, time.Second*5)
	httpServe(metadataListener, s, time.Second*5)
}

func createInterface(bridge net.Interface, threadId int,
	logger log.DebugLogger) error {
	localName := bridge.Name + "-ll"
	remoteName := bridge.Name + "-lr"
	if _, err := net.InterfaceByName(localName); err == nil {
		exec.Command("ip", "link", "delete", localName).Run()
	}
	cmd := exec.Command("ip", "link", "add", localName, "type", "veth",
		"peer", "name", remoteName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error creating veth for bridge: %s: %s: %s",
			bridge.Name, err, output)
	}
	cmd = exec.Command("ifconfig", localName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error bringing up local interface: %s: %s: %s",
			localName, err, output)
	}
	remoteInterface, err := net.InterfaceByName(remoteName)
	if err != nil {
		return err
	}
	cmd = exec.Command("ip", "link", "set", remoteName, "netns",
		strconv.FormatInt(int64(threadId), 10), "name", "eth0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error moving interface to namespace: %s: %s: %s",
			remoteName, err, output)
	}
	cmd = exec.Command("ip", "link", "set", localName, "master", bridge.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error adding interface: %s to bridge: %s: %s: %s",
			localName, bridge.Name, err, output)
	}
	hwAddr := remoteInterface.HardwareAddr.String()
	cmd = exec.Command("ebtables", "-t", "nat", "-A", "PREROUTING",
		"--logical-in", bridge.Name, "-p", "ip",
		"--ip-dst", "169.254.0.0/16", "-j", "dnat", "--to-destination",
		hwAddr)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"error adding ebtables dnat to: %s to bridge: %s: %s: %s",
			hwAddr, bridge.Name, err, output)
	}
	logger.Printf("created veth, remote addr: %s\n", hwAddr)
	return nil
}
