package client

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/queue"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/filegenerator"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

func newManager(objSrv objectserver.ObjectServer, logger log.Logger) *Manager {
	heartbeatChannel := make(chan struct{}, 1)
	sourceReconnectChannel := make(chan string, 1)
	var lastHeartbeatTime time.Time
	m := &Manager{
		sourceMap:              make(map[string]*sourceType),
		objectServer:           objSrv,
		machineMap:             make(map[string]*machineType),
		addMachineChannel:      make(chan *machineType, 1),
		removeMachineChannel:   make(chan string, 1),
		updateMachineChannel:   make(chan *machineType, 1),
		serverMessageChannel:   make(chan *serverMessageType, 1),
		sourceReconnectChannel: sourceReconnectChannel,
		objectWaiters:          make(map[hash.Hash][]chan<- hash.Hash),
		logger:                 debuglogger.Upgrade(logger)}
	tricorder.RegisterMetric("filegen/client/num-object-waiters",
		&m.numObjectWaiters.value, units.None,
		"number of goroutines waiting for objects")
	go m.manage(sourceReconnectChannel, heartbeatChannel)
	go watchHeartbeat(heartbeatChannel, &lastHeartbeatTime, &m.lostHeartbeat,
		&m.lastLostHeartbeatTime, logger)
	tricorder.RegisterMetric("filegen/client/last-heartbeat-time",
		&lastHeartbeatTime, units.None,
		"last manager heartbeat timestamp")
	return m
}

func (m *Manager) manage(sourceConnectChannel <-chan string,
	heartbeatChannel chan<- struct{}) {
	for {
		timer := time.NewTimer(time.Second)
		select {
		case machine := <-m.addMachineChannel:
			m.addMachine(machine)
		case hostname := <-m.removeMachineChannel:
			m.removeMachine(hostname)
		case machine := <-m.updateMachineChannel:
			m.updateMachine(machine)
		case serverMessage := <-m.serverMessageChannel:
			m.processMessage(serverMessage)
		case sourceName := <-sourceConnectChannel:
			m.processSourceConnect(sourceName)
		case <-timer.C:
		}
		clearTimer(timer)
		select {
		case heartbeatChannel <- struct{}{}:
		default:
		}
	}
}

func (m *Manager) processMessage(serverMessage *serverMessageType) {
	if msg := serverMessage.serverMessage.GetObjectResponse; msg != nil {
		if _, _, err := m.objectServer.AddObject(
			bytes.NewReader(msg.Data), 0, &msg.Hash); err != nil {
			m.logger.Println(err)
		} else {
			if waiters, ok := m.objectWaiters[msg.Hash]; ok {
				for _, channel := range waiters {
					channel <- msg.Hash
				}
				delete(m.objectWaiters, msg.Hash)
			}
		}
	}
	if msg := serverMessage.serverMessage.YieldResponse; msg != nil {
		if machine, ok := m.machineMap[msg.Hostname]; ok {
			m.handleYieldResponse(machine, msg.Files)
		} // else machine no longer known. Drop the message.
	}
}

func (m *Manager) processSourceConnect(sourceName string) {
	source := m.sourceMap[sourceName]
	for _, machine := range m.machineMap {
		if pathnames, ok := machine.sourceToPaths[sourceName]; ok {
			request := &proto.ClientRequest{
				YieldRequest: &proto.YieldRequest{machine.machine, pathnames}}
			source.sendChannel <- request
		}
	}
}

func (m *Manager) getSource(sourceName string) *sourceType {
	source, ok := m.sourceMap[sourceName]
	if ok {
		return source
	}
	source = new(sourceType)
	var peakQueueLength, queueLength uint
	sendChannel, receiveChannel := queue.NewChannelPair[*proto.ClientRequest](
		func(length uint) {
			queueLength = length
			if length > peakQueueLength {
				peakQueueLength = length
			}
		})
	tricorder.RegisterMetric(
		path.Join("filegen/client/sources", sourceName, "current-queue-length"),
		&queueLength,
		units.None,
		"current number of entries in send queue")
	tricorder.RegisterMetric(
		path.Join("filegen/client/sources", sourceName, "peak-queue-length"),
		&peakQueueLength,
		units.None,
		"maximum number of entries in send queue")
	source.sendChannel = sendChannel
	m.sourceMap[sourceName] = source
	go manageSource(sourceName, m.sourceReconnectChannel, receiveChannel,
		m.serverMessageChannel, m.logger)
	return source
}

func (m *Manager) writeHtml(writer io.Writer) {
	lastLostHeartbeatTime := m.lastLostHeartbeatTime
	if m.lostHeartbeat {
		fmt.Fprintf(writer,
			"<font color=\"red\">Lost filegen heartbeat since %s (%s ago)</font><p>",
			lastLostHeartbeatTime.Format(format.TimeFormatSeconds),
			format.Duration(time.Since(lastLostHeartbeatTime)))
	} else if !lastLostHeartbeatTime.IsZero() {
		fmt.Fprintf(writer,
			"<font color=\"salmon\">Previously lost filegen heartbeat at %s (%s ago)</font><p>",
			lastLostHeartbeatTime.Format(format.TimeFormatSeconds),
			format.Duration(time.Since(lastLostHeartbeatTime)))
	}
}

func clearTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func manageSource(sourceName string, sourceReconnectChannel chan<- string,
	clientRequestChannel <-chan *proto.ClientRequest,
	serverMessageChannel chan<- *serverMessageType, logger log.Logger) {
	closeNotifyChannel := make(chan struct{}, 1)
	sleeper := backoffdelay.NewExponential(100*time.Millisecond, time.Minute, 1)
	reconnect := false
	for ; ; sleeper.Sleep() {
		client, err := srpc.DialHTTP("tcp", sourceName, time.Second*15)
		if err != nil {
			logger.Printf("error connecting to: %s: %s\n", sourceName, err)
			continue
		}
		conn, err := client.Call("FileGenerator.Connect")
		if err != nil {
			client.Close()
			logger.Println(err)
			continue
		}
		sleeper.Reset()
		go handleServerMessages(sourceName, conn, serverMessageChannel,
			closeNotifyChannel, logger)
		if reconnect {
			sourceReconnectChannel <- sourceName
		} else {
			reconnect = true
		}
		sendClientRequests(conn, clientRequestChannel, closeNotifyChannel,
			logger)
		conn.Close()
		client.Close()
	}
}

func sendClientRequests(conn *srpc.Conn,
	clientRequestChannel <-chan *proto.ClientRequest,
	closeNotifyChannel <-chan struct{}, logger log.Logger) {
	for {
		select {
		case clientRequest := <-clientRequestChannel:
			if err := conn.Encode(clientRequest); err != nil {
				logger.Printf("error encoding client request: %s\n", err)
				return
			}
			if len(clientRequestChannel) < 1 {
				if err := conn.Flush(); err != nil {
					logger.Printf("error flushing: %s\n", err)
					return
				}
			}
		case <-closeNotifyChannel:
			return
		}
	}
}

func handleServerMessages(sourceName string, decoder srpc.Decoder,
	serverMessageChannel chan<- *serverMessageType,
	closeNotifyChannel chan<- struct{}, logger log.Logger) {
	for {
		var message proto.ServerMessage
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				logger.Printf("connection to source: %s closed\n", sourceName)
			} else {
				logger.Println(err)
			}
			closeNotifyChannel <- struct{}{}
			return
		}
		serverMessageChannel <- &serverMessageType{sourceName, message}
	}
}

func watchHeartbeat(heartbeatChannel <-chan struct{},
	heartbeatTimestamp *time.Time, heartbeatStopped *bool,
	lastHeartbeatLostTime *time.Time, logger log.Logger) {
	timer := time.NewTimer(time.Minute)
	for {
		select {
		case <-timer.C:
			logger.Println("filegen: manager heartbeat stopped")
			*heartbeatStopped = true
			*lastHeartbeatLostTime = time.Now()
		case <-heartbeatChannel:
			if *heartbeatStopped {
				logger.Println("filegen: manager heartbeat resumed")
				*heartbeatStopped = false
			}
			clearTimer(timer)
			timer.Reset(time.Minute)
			*heartbeatTimestamp = time.Now()
		}
	}
}
