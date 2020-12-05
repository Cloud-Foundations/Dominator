package rpcd

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"time"

	jsonlib "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/lib"
)

var (
	readOnly = flag.Bool("readOnly", false,
		"If true, refuse all Fetch and Update requests. For debugging only")
	disableUpdates = flag.Bool("disableUpdates", false,
		"If true, refuse all Update requests. For debugging only")
	disableTriggers = flag.Bool("disableTriggers", false,
		"If true, do not run any triggers. For debugging only")
)

type flusher interface {
	Flush() error
}

func (t *rpcType) Update(conn *srpc.Conn, request sub.UpdateRequest,
	reply *sub.UpdateResponse) error {
	if err := t.getUpdateLock(); err != nil {
		t.logger.Println(err)
		return err
	}
	t.logger.Printf("Update()\n")
	fs := t.fileSystemHistory.FileSystem()
	if request.Wait {
		return t.updateAndUnlock(request, fs.RootDirectoryName())
	}
	go t.updateAndUnlock(request, fs.RootDirectoryName())
	return nil
}

func (t *rpcType) getUpdateLock() error {
	if *readOnly || *disableUpdates {
		return errors.New("Update() rejected due to read-only mode")
	}
	fs := t.fileSystemHistory.FileSystem()
	if fs == nil {
		return errors.New("No file-system history yet")
	}
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	if t.fetchInProgress {
		return errors.New("Fetch() in progress")
	}
	if t.updateInProgress {
		return errors.New("Update() already in progress")
	}
	t.updateInProgress = true
	t.lastUpdateError = nil
	return nil
}

func (t *rpcType) updateAndUnlock(request sub.UpdateRequest,
	rootDirectoryName string) error {
	defer t.clearUpdateInProgress()
	defer t.scannerConfiguration.BoostCpuLimit(t.logger)
	t.disableScannerFunc(true)
	defer t.disableScannerFunc(false)
	startTime := time.Now()
	oldTriggers := &triggers.MergeableTriggers{}
	file, err := os.Open(t.oldTriggersFilename)
	if err == nil {
		decoder := json.NewDecoder(file)
		var trig triggers.Triggers
		err = decoder.Decode(&trig.Triggers)
		file.Close()
		if err == nil {
			oldTriggers.Merge(&trig)
		} else {
			t.logger.Printf("Error decoding old triggers: %s", err.Error())
		}
	}
	if request.Triggers != nil {
		// Merge new triggers into old triggers. This supports initial
		// Domination of a machine and when the old triggers are incomplete.
		oldTriggers.Merge(request.Triggers)
		file, err = os.Create(t.oldTriggersFilename)
		if err == nil {
			writer := bufio.NewWriter(file)
			if err := jsonlib.WriteWithIndent(writer, "    ",
				request.Triggers.Triggers); err != nil {
				t.logger.Printf("Error marshaling triggers: %s", err)
			}
			writer.Flush()
			file.Close()
		}
	}
	var hadTriggerFailures bool
	var fsChangeDuration time.Duration
	var lastUpdateError error
	t.workdirGoroutine.Run(func() {
		hadTriggerFailures, fsChangeDuration, lastUpdateError = lib.Update(
			request, rootDirectoryName, t.objectsDir,
			oldTriggers.ExportTriggers(),
			t.scannerConfiguration.ScanFilter, t.runTriggers, t.logger)
	})
	t.lastUpdateHadTriggerFailures = hadTriggerFailures
	t.lastUpdateError = lastUpdateError
	timeTaken := time.Since(startTime)
	if t.lastUpdateError != nil {
		t.logger.Printf("Update(): last error: %s\n", t.lastUpdateError)
	} else {
		t.rwLock.Lock()
		t.lastSuccessfulImageName = request.ImageName
		t.rwLock.Unlock()
	}
	t.logger.Printf("Update() completed in %s (change window: %s)\n",
		timeTaken, fsChangeDuration)
	return t.lastUpdateError
}

func (t *rpcType) clearUpdateInProgress() {
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	t.updateInProgress = false
}

// Returns true if there were failures.
func (t *rpcType) runTriggers(triggers []*triggers.Trigger, action string,
	logger log.Logger) bool {
	var retval bool
	t.systemGoroutine.Run(func() {
		retval = runTriggers(triggers, action, logger)
	})
	return retval
}

// Returns true if there were failures.
func runTriggers(triggers []*triggers.Trigger, action string,
	logger log.Logger) bool {
	doReboot := false
	hadFailures := false
	needRestart := false
	logPrefix := ""
	if *disableTriggers {
		logPrefix = "Disabled: "
	}
	for _, trigger := range triggers {
		if trigger.DoReboot {
			doReboot = true
			break
		}
	}
	if doReboot {
		if action == "start" {
			logger.Printf("%sRebooting\n", logPrefix)
			if *disableTriggers {
				return hadFailures
			}
			if logger, ok := logger.(flusher); ok {
				logger.Flush()
			}
			time.Sleep(time.Second)
			if !runCommand(logger, "reboot") {
				hadFailures = true
			}
		} else {
			logger.Printf("%sWill reboot on start, skipping %s actions\n",
				logPrefix, action)
		}
		return hadFailures
	}
	for _, trigger := range triggers {
		if trigger.Service == "subd" {
			// Never kill myself, just restart. Must do it last, so that other
			// triggers are started.
			if action == "start" {
				needRestart = true
			}
			continue
		}
		logger.Printf("%sAction: service %s %s\n",
			logPrefix, trigger.Service, action)
		if *disableTriggers {
			continue
		}
		if !runCommand(logger, "service", trigger.Service, action) {
			hadFailures = true
			if trigger.DoReboot && action == "start" {
				doReboot = false
			}
		}
	}
	if needRestart {
		logger.Printf("%sAction: service subd restart\n", logPrefix)
		if !runCommand(logger, "service", "subd", "restart") {
			hadFailures = true
		}
	}
	return hadFailures
}

// Returns true on success, else false.
func runCommand(logger log.Logger, name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	if logs, err := cmd.CombinedOutput(); err != nil {
		errMsg := "error running: " + name
		for _, arg := range args {
			errMsg += " " + arg
		}
		errMsg += ": " + err.Error()
		logger.Printf("error running: %s\n", errMsg)
		logger.Println(string(logs))
		return false
	}
	return true
}
