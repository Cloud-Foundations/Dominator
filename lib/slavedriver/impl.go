package slavedriver

import (
	"container/list"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type jsonDatabase struct {
	filename string
}

func dialWithRetry(network, address string,
	timeout time.Duration) (*srpc.Client, error) {
	stopTime := time.Now().Add(timeout)
	sleeper := backoffdelay.NewExponential(100*time.Millisecond, time.Second, 1)
	for ; time.Until(stopTime) >= 0; sleeper.Sleep() {
		client, err := srpc.DialHTTP(network, address, time.Second)
		if err != nil {
			continue
		}
		if err := client.SetKeepAlivePeriod(time.Second * 30); err != nil {
			client.Close()
			return nil, err
		}
		return client, nil

	}
	return nil, fmt.Errorf("timed out connecting to: %s", address)
}

func listSlaves(slaves map[*Slave]struct{}) []SlaveInfo {
	list := make([]SlaveInfo, 0, len(slaves))
	for slave := range slaves {
		list = append(list, slave.info)
	}
	return list
}

func newSlaveDriver(options SlaveDriverOptions, slaveTrader SlaveTrader,
	clientDialer clientDialerFunc, databaseDriver databaseLoadSaver,
	logger log.DebugLogger) (*SlaveDriver, error) {
	if options.MinimumIdleSlaves < 1 {
		options.MinimumIdleSlaves = 1
	}
	if options.MaximumIdleSlaves < 1 {
		options.MaximumIdleSlaves = 1
	}
	if options.MaximumIdleSlaves < options.MinimumIdleSlaves {
		options.MaximumIdleSlaves = options.MinimumIdleSlaves
	}
	destroySlaveChannel := make(chan *Slave, 1)
	getSlaveChannel := make(chan requestSlaveMessage)
	getSlavesChannel := make(chan chan<- slaveRoll)
	releaseSlaveChannel := make(chan *Slave, 1)
	replaceIdleChannel := make(chan bool)
	publicDriver := &SlaveDriver{
		options:             options,
		destroySlaveChannel: destroySlaveChannel,
		getSlaveChannel:     getSlaveChannel,
		getSlavesChannel:    getSlavesChannel,
		logger:              logger,
		releaseSlaveChannel: releaseSlaveChannel,
		replaceIdleChannel:  replaceIdleChannel,
	}
	driver := &slaveDriver{
		options:             options,
		busySlaves:          make(map[*Slave]struct{}),
		clientDialer:        clientDialer,
		destroySlaveChannel: destroySlaveChannel,
		databaseDriver:      databaseDriver,
		getSlaveChannel:     getSlaveChannel,
		getSlavesChannel:    getSlavesChannel,
		getterList:          list.New(),
		logger:              logger,
		publicDriver:        publicDriver,
		slaveTrader:         slaveTrader,
		releaseSlaveChannel: releaseSlaveChannel,
		replaceIdleChannel:  replaceIdleChannel,
	}
	if err := driver.loadSlaves(); err != nil {
		driver.slaveTrader.Close()
		return nil, err
	}
	go driver.watchRoll()
	return publicDriver, nil
}

func (db *jsonDatabase) load() (*slaveRoll, error) {
	var slaves slaveRoll
	err := json.ReadFromFile(db.filename, &slaves)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &slaves, nil
}

func (db *jsonDatabase) save(slaves slaveRoll) error {
	return json.WriteToFile(db.filename, fsutil.PublicFilePerms, "    ", slaves)
}

func (slave *Slave) getClient() *srpc.Client {
	return slave.client
}

func (driver *SlaveDriver) getSlave(timeout time.Duration) (*Slave, error) {
	driver.logger.Debugln(0, "getSlave() starting")
	if timeout < 0 {
		timeout = time.Hour
	}
	slaveChannel := make(chan *Slave)
	driver.getSlaveChannel <- requestSlaveMessage{
		slaveChannel: slaveChannel,
		timeout:      time.Now().Add(timeout),
	}
	if slave := <-slaveChannel; slave == nil {
		return nil, fmt.Errorf("timed out getting slave")
	} else {
		return slave, nil
	}
}

func (driver *slaveDriver) createSlave() {
	driver.logger.Debugln(0, "creating slave")
	sleeper := backoffdelay.NewExponential(time.Second, time.Minute, 1)
	for ; ; sleeper.Sleep() {
		slaveInfo, err := driver.slaveTrader.CreateSlave()
		if err != nil {
			driver.logger.Println(err)
			continue
		}
		slave := &Slave{
			clientAddress: fmt.Sprintf("%s:%d", slaveInfo.IpAddress,
				driver.options.PortNumber),
			info:   slaveInfo,
			driver: driver.publicDriver,
		}
		slave.client, err = driver.clientDialer("tcp", slave.clientAddress,
			time.Minute)
		if err != nil {
			e := driver.slaveTrader.DestroySlave(slaveInfo.Identifier)
			if e != nil {
				driver.logger.Printf("error destroying: %s: %s\n",
					slaveInfo.Identifier, e)
			}
			driver.logger.Printf("error dialing: %s: %s\n",
				slave.clientAddress, err)
			continue
		}
		driver.logger.Printf("created slave: %s\n", slaveInfo.Identifier)
		driver.createdSlaveChannel <- slave
		return
	}
}

func (driver *slaveDriver) getSlaves() slaveRoll {
	return slaveRoll{
		BusySlaves: listSlaves(driver.busySlaves),
		IdleSlaves: listSlaves(driver.idleSlaves),
		Zombies:    listSlaves(driver.zombies),
	}
}

func (driver *slaveDriver) loadSlaves() error {
	slavesFromDB, err := driver.databaseDriver.load()
	if err != nil {
		return err
	}
	if slavesFromDB == nil {
		driver.idleSlaves = make(map[*Slave]struct{})
		driver.zombies = make(map[*Slave]struct{})
		return nil
	}
	slavesFromDB.BusySlaves = append(slavesFromDB.BusySlaves,
		slavesFromDB.Zombies...)
	driver.idleSlaves = make(map[*Slave]struct{}, len(slavesFromDB.IdleSlaves))
	driver.zombies = make(map[*Slave]struct{}, len(slavesFromDB.BusySlaves))
	for _, slaveInfo := range slavesFromDB.BusySlaves {
		driver.zombies[&Slave{
			driver: driver.publicDriver,
			info:   slaveInfo,
		}] = struct{}{}
	}
	for _, slaveInfo := range slavesFromDB.IdleSlaves {
		slave := &Slave{
			clientAddress: fmt.Sprintf("%s:%d", slaveInfo.IpAddress,
				driver.options.PortNumber),
			info:   slaveInfo,
			driver: driver.publicDriver,
		}
		slave.client, err = driver.clientDialer("tcp", slave.clientAddress,
			time.Minute)
		if err != nil {
			driver.logger.Printf("error dialing: %s: %s\n", slave.clientAddress,
				err)
			driver.zombies[slave] = struct{}{}
		} else {
			driver.idleSlaves[slave] = struct{}{}
		}
	}
	return nil
}

// rollCall manages all the internal state. It should be called from a forever
// goroutine.
func (driver *slaveDriver) rollCall() {
	driver.logger.Debugf(0, "rollCall(): %d idle, %d getters\n",
		len(driver.idleSlaves), driver.getterList.Len())
	// First: if there is an idle slave, dispatch to a getter.
	if len(driver.idleSlaves) > 0 && driver.getterList.Len() > 0 {
		entry := driver.getterList.Front()
		request := entry.Value.(requestSlaveMessage)
		driver.getterList.Remove(entry)
		if time.Since(request.timeout) > 0 {
			request.slaveChannel <- nil // Getter wanted to give up by now.
			close(request.slaveChannel)
			return
		}
		for slave := range driver.idleSlaves {
			request.slaveChannel <- slave // Consumed by getter.
			close(request.slaveChannel)
			delete(driver.idleSlaves, slave)
			driver.busySlaves[slave] = struct{}{}
			driver.writeState = true
			driver.logger.Debugf(0, "sent slave: %s to getter\n", slave)
			return
		}
	}
	if driver.getterList.Len() > 0 ||
		uint(len(driver.idleSlaves)) < driver.options.MinimumIdleSlaves {
		if driver.createdSlaveChannel == nil {
			driver.createdSlaveChannel = make(chan *Slave, 1)
			go driver.createSlave()
		}
	}
	if uint(len(driver.idleSlaves)) > driver.options.MaximumIdleSlaves &&
		driver.getterList.Len() < 1 {
		for slave := range driver.idleSlaves {
			if uint(len(driver.idleSlaves)) <=
				driver.options.MaximumIdleSlaves {
				break
			}
			delete(driver.idleSlaves, slave)
			driver.zombies[slave] = struct{}{}
			driver.writeState = true
		}
	}
	for slave := range driver.zombies {
		driver.logger.Printf("destroying slave: %s\n", slave.info.Identifier)
		err := driver.slaveTrader.DestroySlave(slave.info.Identifier)
		if err != nil {
			driver.logger.Printf("error destroying: %s: %s\n",
				slave.info.Identifier, err)
		} else {
			delete(driver.zombies, slave)
			driver.writeState = true
		}
	}
	if driver.writeState {
		if err := driver.databaseDriver.save(driver.getSlaves()); err != nil {
			driver.logger.Println(err)
		} else {
			driver.writeState = false
		}
	}
	select {
	case slave := <-driver.createdSlaveChannel:
		driver.createdSlaveChannel = nil
		driver.idleSlaves[slave] = struct{}{}
		driver.writeState = true
		return // Return now so that new slave can be sent to a getter quickly.
	case slave := <-driver.destroySlaveChannel:
		if _, ok := driver.idleSlaves[slave]; ok {
			panic("destroying idle slave")
		}
		if _, ok := driver.zombies[slave]; ok {
			panic("destroying zombie")
		}
		if _, ok := driver.busySlaves[slave]; !ok {
			panic("destroying unknown slave")
		}
		delete(driver.busySlaves, slave)
		driver.zombies[slave] = struct{}{}
		driver.writeState = true
	case slaveChannel := <-driver.getSlaveChannel:
		driver.getterList.PushBack(slaveChannel)
	case slavesChannel := <-driver.getSlavesChannel:
		slavesChannel <- driver.getSlaves()
	case slave := <-driver.releaseSlaveChannel:
		if _, ok := driver.idleSlaves[slave]; ok {
			panic("releasing idle slave")
		}
		if _, ok := driver.zombies[slave]; ok {
			panic("releasing zombie")
		}
		if _, ok := driver.busySlaves[slave]; !ok {
			panic("releasing unknown slave")
		}
		delete(driver.busySlaves, slave)
		driver.idleSlaves[slave] = struct{}{}
		driver.writeState = true
	case createIfNeeded := <-driver.replaceIdleChannel:
		for slave := range driver.idleSlaves {
			delete(driver.idleSlaves, slave)
			driver.zombies[slave] = struct{}{}
			driver.writeState = true
		}
		if createIfNeeded && driver.createdSlaveChannel == nil {
			driver.createdSlaveChannel = make(chan *Slave, 1)
			go driver.createSlave()
		}
	}
}

func (driver *slaveDriver) watchRoll() {
	for {
		driver.rollCall()
	}
}

func (driver *SlaveDriver) writeHtml(writer io.Writer) {
	slavesChannel := make(chan slaveRoll)
	driver.getSlavesChannel <- slavesChannel
	slaves := <-slavesChannel
	if len(slaves.BusySlaves) < 1 && len(slaves.IdleSlaves) < 1 &&
		len(slaves.Zombies) < 1 {
		fmt.Fprintf(writer, "No slaves for %s<br>\n", driver.options.Purpose)
		return
	}
	fmt.Fprintf(writer, "Slaves for %s:<br>\n", driver.options.Purpose)
	for _, slave := range slaves.BusySlaves {
		fmt.Fprintf(writer,
			"&nbsp;&nbsp;<a href=\"http://%s:%d/\">%s</a> (busy)<br>\n",
			slave.IpAddress, driver.options.PortNumber, slave)
	}
	for _, slave := range slaves.IdleSlaves {
		fmt.Fprintf(writer,
			"&nbsp;&nbsp;<a href=\"http://%s:%d/\">%s</a> (idle)<br>\n",
			slave.IpAddress, driver.options.PortNumber, slave)
	}
	for _, slave := range slaves.Zombies {
		fmt.Fprintf(writer,
			"&nbsp;&nbsp;<a href=\"http://%s:%d/\">%s</a> (zombie)<br>\n",
			slave.IpAddress, driver.options.PortNumber, slave)
	}
}
