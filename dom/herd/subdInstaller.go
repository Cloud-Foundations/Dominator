package herd

import (
	"bytes"
	"os/exec"
	"runtime"
	"time"
)

var (
	carriageReturnLiteral = []byte{'\r'}
	newlineLiteral        = []byte{'\n'}
	newlineReplacement    = []byte{'\\', 'n'}
)

type completionType struct {
	failed   bool
	hostname string
}

type installerQueueType struct {
	entries map[string]*queueEntry // Key: subHostname (nil: processing).
	first   *queueEntry
	last    *queueEntry
}

type queueEntry struct {
	startTime time.Time
	hostname  string
	prev      *queueEntry
	next      *queueEntry
}

func (herd *Herd) subdInstallerLoop() {
	if *subdInstaller == "" {
		return
	}
	availableSlots := runtime.NumCPU()
	completion := make(chan completionType, 1)
	queueAdd := make(chan string, 1)
	herd.subdInstallerQueueAdd = queueAdd
	queueDelete := make(chan string, 1)
	herd.subdInstallerQueueDelete = queueDelete
	queueErase := make(chan string, 1)
	herd.subdInstallerQueueErase = queueErase
	queue := installerQueueType{entries: make(map[string]*queueEntry)}
	for {
		sleepInterval := time.Hour
		if queue.first != nil && availableSlots > 0 {
			sleepInterval = time.Until(queue.first.startTime)
		}
		timer := time.NewTimer(sleepInterval)
		select {
		case <-timer.C:
		case hostname := <-queueAdd:
			if _, ok := queue.entries[hostname]; !ok {
				entry := &queueEntry{
					startTime: time.Now().Add(*subdInstallDelay),
					hostname:  hostname,
					prev:      queue.last,
				}
				queue.add(entry)
			}
		case hostname := <-queueDelete:
			if entry := queue.entries[hostname]; entry != nil {
				queue.delete(entry)
				delete(queue.entries, hostname)
			}
		case hostname := <-queueErase:
			if entry := queue.entries[hostname]; entry != nil {
				queue.delete(entry)
			}
			delete(queue.entries, hostname)
		case result := <-completion:
			availableSlots++
			if _, ok := queue.entries[result.hostname]; ok { // Not erased.
				delete(queue.entries, result.hostname)
				if result.failed && *subdInstallRetryDelay > *subdInstallDelay {
					// Come back later rather than sooner, must re-inject now.
					entry := &queueEntry{
						startTime: time.Now().Add(*subdInstallRetryDelay),
						hostname:  result.hostname,
						prev:      queue.last,
					}
					queue.add(entry)
				}
			}
		}
		timer.Stop()
		entry := queue.first
		if entry != nil &&
			availableSlots > 0 &&
			time.Since(entry.startTime) >= 0 {
			availableSlots--
			go herd.subInstall(entry.hostname, completion)
			queue.delete(entry)
			queue.entries[entry.hostname] = nil // Mark as processing.
		}
	}
}

func (herd *Herd) addSubToInstallerQueue(subHostname string) {
	if herd.subdInstallerQueueAdd != nil {
		herd.subdInstallerQueueAdd <- subHostname
	}
}

func (herd *Herd) eraseSubFromInstallerQueue(subHostname string) {
	if herd.subdInstallerQueueErase != nil {
		herd.subdInstallerQueueErase <- subHostname
	}
}

func (herd *Herd) removeSubFromInstallerQueue(subHostname string) {
	if herd.subdInstallerQueueDelete != nil {
		herd.subdInstallerQueueDelete <- subHostname
	}
}

func (herd *Herd) subInstall(subHostname string,
	completion chan<- completionType) {
	failed := false
	defer func() { completion <- completionType{failed, subHostname} }()
	herd.logger.Printf("Installing subd on: %s\n", subHostname)
	cmd := exec.Command(*subdInstaller, subHostname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		failed = true
		if len(output) > 0 && output[len(output)-1] == '\n' {
			output = output[:len(output)-1]
		}
		output = bytes.ReplaceAll(output, carriageReturnLiteral, nil)
		output = bytes.ReplaceAll(output, newlineLiteral, newlineReplacement)
		herd.logger.Printf("Error installing subd on: %s: %s: %s\n",
			subHostname, err, string(output))
	}
}

func (queue *installerQueueType) add(entry *queueEntry) {
	entry.prev = queue.last
	if queue.first == nil {
		queue.first = entry
	} else {
		queue.last.next = entry
	}
	queue.last = entry
	queue.entries[entry.hostname] = entry
}

func (queue *installerQueueType) delete(entry *queueEntry) {
	if entry.prev == nil {
		queue.first = entry.next
	} else {
		entry.prev.next = entry.next
	}
	if entry.next == nil {
		queue.last = entry.prev
	} else {
		entry.next.prev = entry.prev
	}
}
