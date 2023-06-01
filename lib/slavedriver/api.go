package slavedriver

import (
	"container/list"
	"io"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type Slave struct {
	clientAddress string
	driver        *SlaveDriver
	info          SlaveInfo
	client        *srpc.Client
	timeToPing    time.Time
	pinging       bool
}

func (slave *Slave) Destroy() {
	slave.driver.destroySlaveChannel <- slave
}

func (slave *Slave) GetClient() *srpc.Client {
	return slave.getClient()
}

func (slave *Slave) GetClientAddress() string {
	return slave.clientAddress
}

func (slave *Slave) Release() {
	slave.driver.releaseSlaveChannel <- slave
}

func (slave *Slave) String() string {
	return slave.info.Identifier
}

type SlaveDriver struct {
	options             SlaveDriverOptions
	destroySlaveChannel chan<- *Slave
	getSlaveChannel     chan<- requestSlaveMessage
	getSlavesChannel    chan<- chan<- slaveRoll
	logger              log.DebugLogger
	releaseSlaveChannel chan<- *Slave
	replaceIdleChannel  chan<- bool
}

func NewSlaveDriver(options SlaveDriverOptions, slaveTrader SlaveTrader,
	logger log.DebugLogger) (*SlaveDriver, error) {
	return newSlaveDriver(options, slaveTrader,
		dialWithRetry, &jsonDatabase{options.DatabaseFilename}, logger)
}

func (driver *SlaveDriver) GetSlave() (*Slave, error) {
	return driver.getSlave(-1)
}

func (driver *SlaveDriver) GetSlaveWithTimeout(timeout time.Duration) (
	*Slave, error) {
	return driver.getSlave(timeout)
}

func (driver *SlaveDriver) ReplaceIdle(immediateGetNew bool) {
	driver.replaceIdleChannel <- immediateGetNew
}

func (driver *SlaveDriver) WriteHtml(writer io.Writer) {
	driver.writeHtml(writer)
}

type SlaveDriverOptions struct {
	DatabaseFilename  string
	MaximumIdleSlaves uint
	MinimumIdleSlaves uint
	PortNumber        uint
	Purpose           string
}

type SlaveInfo struct {
	Identifier string
	IpAddress  net.IP
}

func (si SlaveInfo) String() string {
	return si.Identifier
}

type SlaveTrader interface {
	Close() error
	CreateSlave() (SlaveInfo, error)
	DestroySlave(identifier string) error
}

type clientDialerFunc func(string, string, time.Duration) (*srpc.Client, error)

type databaseLoadSaver interface {
	load() (*slaveRoll, error)
	save(slaveRoll) error
}

type pingResponseMessage struct {
	error error
	slave *Slave
}

type requestSlaveMessage struct {
	slaveChannel chan<- *Slave
	timeout      time.Time
}

type slaveDriver struct {
	options               SlaveDriverOptions
	busySlaves            map[*Slave]struct{}
	clientDialer          clientDialerFunc
	createdSlaveChannel   chan *Slave
	destroySlaveChannel   <-chan *Slave
	destroyedSlaveChannel chan *Slave
	databaseDriver        databaseLoadSaver
	getSlaveChannel       <-chan requestSlaveMessage
	getSlavesChannel      <-chan chan<- slaveRoll
	getterList            *list.List
	idleSlaves            map[*Slave]struct{}
	logger                log.DebugLogger
	pingResponseChannel   chan pingResponseMessage
	publicDriver          *SlaveDriver
	releaseSlaveChannel   <-chan *Slave
	replaceIdleChannel    <-chan bool
	slaveTrader           SlaveTrader
	writeState            bool
	zombies               map[*Slave]struct{}
}

type slaveRoll struct {
	BusySlaves []SlaveInfo `json:",omitempty"`
	IdleSlaves []SlaveInfo `json:",omitempty"`
	Zombies    []SlaveInfo `json:",omitempty"`
}
