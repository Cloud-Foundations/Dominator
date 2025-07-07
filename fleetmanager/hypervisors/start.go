package hypervisors

import (
	"os"
	"runtime"

	"github.com/Cloud-Foundations/Dominator/lib/html"
)

func newManager(startOptions StartOptions) (*Manager, error) {
	if err := checkPoolLimits(); err != nil {
		return nil, err
	}
	if startOptions.IpmiPasswordFile != "" {
		file, err := os.Open(startOptions.IpmiPasswordFile)
		if err != nil {
			return nil, err
		}
		file.Close()
	}
	manager := &Manager{
		ipmiLimiter:      make(chan struct{}, runtime.NumCPU()),
		ipmiPasswordFile: startOptions.IpmiPasswordFile,
		ipmiUsername:     startOptions.IpmiUsername,
		logger:           startOptions.Logger,
		storer:           startOptions.Storer,
		allocatingIPs:    make(map[string]struct{}),
		hypervisors:      make(map[string]*hypervisorType),
		hypervisorsByHW:  make(map[string]*hypervisorType),
		hypervisorsByIP:  make(map[string]*hypervisorType),
		migratingIPs:     make(map[string]struct{}),
		subnets:          make(map[string]*subnetType),
		vms:              make(map[string]*vmInfoType),
	}
	html.HandleFunc("/listHypervisors", manager.listHypervisorsHandler)
	html.HandleFunc("/listLocations", manager.listLocationsHandler)
	html.HandleFunc("/listVMs", manager.listVMsHandler)
	html.HandleFunc("/listVMsByPrimaryOwner",
		manager.listVMsByPrimaryOwnerHandler)
	html.HandleFunc("/showHypervisor", manager.showHypervisorHandler)
	html.HandleFunc("/showVM", manager.showVmHandler)
	html.HandleFunc("/tftpdata/config.json", manager.tftpdataConfigHandler)
	if *manageHypervisors {
		html.HandleFunc("/tftpdata/imagename", manager.tftpdataImageNameHandler)
	}
	go manager.notifierLoop()
	return manager, nil
}
