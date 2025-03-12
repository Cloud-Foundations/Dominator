package filegen

import (
	"bytes"
	"path"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
	proto "github.com/Cloud-Foundations/Dominator/proto/filegenerator"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

type hashGenerator interface {
	generate(machine mdb.Machine, logger log.Logger) (
		hashVal hash.Hash, length uint64, validUntil time.Time, err error)
}

type hashGeneratorWrapper struct {
	dataGenerator FileGenerator
	objectServer  *memory.ObjectServer
}

func (m *Manager) registerDataGeneratorForPath(pathname string,
	gen FileGenerator) chan<- string {
	hashGenerator := &hashGeneratorWrapper{gen, m.objectServer}
	return m.registerHashGeneratorForPath(pathname, hashGenerator)
}

func (m *Manager) registerHashGeneratorForPath(pathname string,
	gen hashGenerator) chan<- string {
	m.rwMutex.RLock()
	_, ok := m.pathManagers[pathname]
	m.rwMutex.RUnlock()
	if ok {
		panic(pathname + " already registered")
	}
	notifyChan := make(chan string, 1)
	pathMgr := &pathManager{
		distributionFailed:     m.bucketer.NewCumulativeDistribution(),
		distributionSuccessful: m.bucketer.NewCumulativeDistribution(),
		generator:              gen,
		machineHashes:          make(map[string]expiringHash)}
	err := tricorder.RegisterMetric(
		path.Join("filegen/generators", pathname, "failed-durations"),
		pathMgr.distributionFailed,
		units.Millisecond,
		"duration of failed generator calls")
	if err != nil {
		panic(err)
	}
	err = tricorder.RegisterMetric(
		path.Join("filegen/generators", pathname, "successful-durations"),
		pathMgr.distributionSuccessful,
		units.Millisecond,
		"duration of successful generator calls")
	if err != nil {
		panic(err)
	}
	m.rwMutex.Lock()
	m.pathManagers[pathname] = pathMgr
	m.rwMutex.Unlock()
	go m.processPathDataInvalidations(pathname, notifyChan)
	return notifyChan
}

func (m *Manager) processPathDataInvalidations(pathname string,
	machineNameChannel <-chan string) {
	m.rwMutex.RLock()
	pathMgr := m.pathManagers[pathname]
	m.rwMutex.RUnlock()
	for machineName := range machineNameChannel {
		if machineName == "" {
			m.rwMutex.RLock()
			for _, mdbData := range m.machineData {
				hashVal, length, validUntil, err := pathMgr.generate(mdbData,
					m.logger)
				if err != nil {
					continue
				}
				pathMgr.rwMutex.Lock()
				pathMgr.machineHashes[mdbData.Hostname] = expiringHash{
					hashVal, length, validUntil}
				pathMgr.rwMutex.Unlock()
				files := make([]proto.FileInfo, 1)
				files[0].Pathname = pathname
				files[0].Hash = hashVal
				files[0].Length = length
				yieldResponse := &proto.YieldResponse{mdbData.Hostname, files}
				message := &proto.ServerMessage{YieldResponse: yieldResponse}
				for _, clientChannel := range m.clients {
					clientChannel <- message
				}
				m.scheduleTimer(pathname, mdbData.Hostname, validUntil)
			}
			m.rwMutex.RUnlock()
		} else {
			m.rwMutex.RLock()
			mdbData := m.machineData[machineName]
			m.rwMutex.RUnlock()
			hashVal, length, validUntil, err := pathMgr.generate(mdbData,
				m.logger)
			if err != nil {
				continue
			}
			pathMgr.rwMutex.Lock()
			pathMgr.machineHashes[machineName] = expiringHash{
				hashVal, length, validUntil}
			pathMgr.rwMutex.Unlock()
			files := make([]proto.FileInfo, 1)
			files[0].Pathname = pathname
			files[0].Hash = hashVal
			files[0].Length = length
			yieldResponse := &proto.YieldResponse{machineName, files}
			message := &proto.ServerMessage{YieldResponse: yieldResponse}
			for _, clientChannel := range m.clients {
				clientChannel <- message
			}
			m.scheduleTimer(pathname, machineName, validUntil)
		}
	}
}

func (m *Manager) scheduleTimer(pathname string, hostname string,
	validUntil time.Time) {
	if validUntil.IsZero() || time.Now().After(validUntil) {
		return // No expiration or already expired.
	}
	m.rwMutex.RLock()
	pathMgr := m.pathManagers[pathname]
	m.rwMutex.RUnlock()
	time.AfterFunc(validUntil.Sub(time.Now()), func() {
		m.rwMutex.RLock()
		mdbData, ok := m.machineData[hostname]
		m.rwMutex.RUnlock()
		if !ok {
			return
		}
		hashVal, length, validUntil, err := pathMgr.generate(mdbData, m.logger)
		if err != nil {
			m.logger.Printf("Error regenerating path: %s for machine: %s: %s\n",
				pathname, hostname, err)
			m.scheduleTimer(pathname, hostname,
				time.Now().Add(generateFailureRetryInterval))
			return
		}
		pathMgr.rwMutex.Lock()
		pathMgr.machineHashes[hostname] = expiringHash{
			hashVal, length, validUntil}
		pathMgr.rwMutex.Unlock()
		files := make([]proto.FileInfo, 1)
		files[0].Pathname = pathname
		files[0].Hash = hashVal
		files[0].Length = length
		yieldResponse := &proto.YieldResponse{mdbData.Hostname, files}
		message := &proto.ServerMessage{YieldResponse: yieldResponse}
		for _, clientChannel := range m.clients {
			clientChannel <- message
		}
		m.logger.Debugf(1, "scheduleTimer: machine: %s, path: %s, hash: %0x\n",
			hostname, pathname, hashVal)
		m.scheduleTimer(pathname, mdbData.Hostname, validUntil)
	})
}

func (m *Manager) getRegisteredPaths() []string {
	m.rwMutex.RLock()
	pathnames := make([]string, 0, len(m.pathManagers))
	for pathname := range m.pathManagers {
		pathnames = append(pathnames, pathname)
	}
	m.rwMutex.RUnlock()
	sort.Strings(pathnames)
	return pathnames
}

func (p *pathManager) generate(machine mdb.Machine, logger log.Logger) (
	hash.Hash, uint64, time.Time, error) {
	startTime := time.Now()
	hashVal, length, expiresAt, err := p.generator.generate(machine, logger)
	timeTaken := time.Since(startTime)
	if err == nil {
		p.distributionSuccessful.Add(timeTaken)
	} else {
		p.distributionFailed.Add(timeTaken)
	}
	return hashVal, length, expiresAt, err
}

func (g *hashGeneratorWrapper) generate(machine mdb.Machine,
	logger log.Logger) (
	hash.Hash, uint64, time.Time, error) {
	data, validUntil, err := g.dataGenerator.Generate(machine, logger)
	if err != nil {
		return hash.Hash{}, 0, time.Time{}, err
	}
	length := uint64(len(data))
	hashVal, _, err := g.objectServer.AddObject(bytes.NewReader(data), length,
		nil)
	if err != nil {
		return hash.Hash{}, 0, time.Time{}, err
	}
	return hashVal, length, validUntil, nil
}
