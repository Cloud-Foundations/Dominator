package rpcd

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	jsonlib "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/osutil"
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
	if err := t.getUpdateLock(conn); err != nil {
		t.params.Logger.Println(err)
		return err
	}
	t.params.Logger.Printf("Update(%s)\n", conn.Username())
	fs := t.params.FileSystemHistory.FileSystem()
	if request.Wait {
		return t.updateAndUnlock(request, fs.RootDirectoryName())
	}
	go t.updateAndUnlock(request, fs.RootDirectoryName())
	return nil
}

func (t *rpcType) getUpdateLock(conn *srpc.Conn) error {
	if *readOnly || *disableUpdates {
		return errors.New("Update() rejected due to read-only mode")
	}
	fs := t.params.FileSystemHistory.FileSystem()
	if fs == nil {
		return errors.New("no file-system history yet")
	}
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	if err := t.checkIfLockedByAnotherClient(conn); err != nil {
		t.params.Logger.Printf("Error: %s\n", err)
		return err
	}
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
	defer t.params.ScannerConfiguration.BoostCpuLimit(t.params.Logger)
	t.params.DisableScannerFunction(true)
	defer t.params.DisableScannerFunction(false)
	startTime := time.Now()
	oldTriggers := &triggers.MergeableTriggers{}
	file, err := os.Open(t.config.OldTriggersFilename)
	if err == nil {
		decoder := json.NewDecoder(file)
		var trig triggers.Triggers
		err = decoder.Decode(&trig.Triggers)
		file.Close()
		if err == nil {
			oldTriggers.Merge(&trig)
		} else {
			t.params.Logger.Printf(
				"Error decoding old triggers: %s", err.Error())
		}
	}
	if request.Triggers != nil {
		// Merge new triggers into old triggers. This supports initial
		// Domination of a machine and when the old triggers are incomplete.
		oldTriggers.Merge(request.Triggers)
		file, err = os.Create(t.config.OldTriggersFilename)
		if err == nil {
			writer := bufio.NewWriter(file)
			if err := jsonlib.WriteWithIndent(writer, "    ",
				request.Triggers.Triggers); err != nil {
				t.params.Logger.Printf("Error marshaling triggers: %s", err)
			}
			writer.Flush()
			file.Close()
		}
	}
	var hadTriggerFailures bool
	var fsChangeDuration time.Duration
	var lastUpdateError error
	options := lib.UpdateOptions{
		Logger:            t.params.Logger,
		ObjectsDir:        t.config.ObjectsDirectoryName,
		OldTriggers:       oldTriggers.ExportTriggers(),
		RootDirectoryName: rootDirectoryName,
		RunTriggers:       t.runTriggers,
		SkipFilter:        t.params.ScannerConfiguration.ScanFilter,
	}
	if t.config.DisruptionManager != "" {
		options.DisruptionCancel = t.disruptionCancel
		options.DisruptionRequest = t.disruptionRequest
	}
	t.stoppedServices = make(map[string]struct{})
	t.params.WorkdirGoroutine.Run(func() {
		hadTriggerFailures, fsChangeDuration, lastUpdateError =
			lib.UpdateWithOptions(request, options)
	})
	t.lastUpdateHadTriggerFailures = hadTriggerFailures
	t.lastUpdateError = lastUpdateError
	timeTaken := time.Since(startTime)
	t.stoppedServices = nil
	if t.lastUpdateError != nil {
		t.params.Logger.Printf("Update(): last error: %s\n", t.lastUpdateError)
	} else {
		note, err := t.generateNote()
		if err != nil {
			t.params.Logger.Println(err)
		}
		t.rwLock.Lock()
		if !request.SparseImage {
			t.lastSuccessfulImageName = request.ImageName
		}
		if err == nil {
			t.lastNote = note
		}
		t.rwLock.Unlock()
	}
	t.params.Logger.Printf("Update() completed in %s (change window: %s)\n",
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
		retval = runTriggers(triggers, action, t.stoppedServices, logger)
	})
	return retval
}

func forceRebootAndWait(logger log.Logger) {
	failureChannel := osutil.RunCommandBackground(logger, "reboot", "-f")
	timer := time.NewTimer(15 * time.Second)
	select {
	case <-failureChannel:
		logger.Printf("Force reboot failed, rebooting harder\n")
	case <-timer.C:
		logger.Printf("Still alive after 15 seconds, rebooting harder\n")
	}
	if logger, ok := logger.(flusher); ok {
		logger.Flush()
	}
}

func handleSignals(signals <-chan os.Signal, logger log.Logger) {
	for sig := range signals {
		logger.Printf("Caught %s: ignoring\n", sig)
		if logger, ok := logger.(flusher); ok {
			logger.Flush()
		}
	}
}

func normalRebootAndWait(logger log.Logger) {
	failureChannel := osutil.RunCommandBackground(logger, "reboot")
	timer := time.NewTimer(time.Minute)
	select {
	case <-failureChannel:
		logger.Printf("Reboot failed, forcing reboot\n")
	case <-timer.C:
		logger.Printf("Still alive after 1 minute, forcing reboot\n")
	}
	if logger, ok := logger.(flusher); ok {
		logger.Flush()
	}
}

// Returns true if there were failures.
func runTriggers(triggerList []*triggers.Trigger, action string,
	stoppedServices map[string]struct{}, logger log.Logger) bool {
	hadFailures := false
	needRestart := false
	logPrefix := ""
	var rebootingTriggers []*triggers.Trigger
	restartingTriggers := make(map[string]struct{})
	if *disableTriggers {
		logPrefix = "Disabled: "
	}
	for _, trigger := range triggerList {
		if trigger.DoReboot {
			rebootingTriggers = append(rebootingTriggers, trigger)
		}
		if !trigger.DoReload {
			restartingTriggers[trigger.Service] = struct{}{}
		}
	}
	if len(rebootingTriggers) > 0 {
		if action == "start" {
			triggerList = rebootingTriggers
		} else {
			logger.Printf("%sWill reboot on start, skipping %s actions\n",
				logPrefix, action)
			return hadFailures
		}
	}
	for _, trigger := range triggerList {
		if trigger.Service == "subd" {
			// Never kill myself, just restart. Must do it last, so that other
			// triggers are started.
			if action == "start" {
				needRestart = true
			}
			continue
		}
		action := action
		if _, ok := restartingTriggers[trigger.Service]; ok {
			if trigger.DoReload {
				continue // This service will be stopped/started: skip reload.
			}
			if action == "stop" {
				stoppedServices[trigger.Service] = struct{}{}
			}
		} else if _, stopped := stoppedServices[trigger.Service]; !stopped {
			// This service only needs to be reloaded.
			if action == "stop" {
				continue // Skip stopping the service.
			}
			if len(rebootingTriggers) > 0 {
				continue // We're going to reboot anyway: skip reloading.
			}
			action = "reload"
		}
		logger.Printf("%sAction: service %s %s\n",
			logPrefix, trigger.Service, action)
		if *disableTriggers {
			continue
		}
		if !osutil.RunCommand(logger, "service", trigger.Service, action) {
			// Ignore start failure for the "reboot" service: try later.
			if action == "start" &&
				trigger.DoReboot &&
				trigger.Service == "reboot" {
				continue
			}
			hadFailures = true
		}
	}
	if len(rebootingTriggers) > 0 {
		if hadFailures {
			logger.Printf("%sSome triggers failed, will not reboot\n",
				logPrefix)
			return hadFailures
		}
		logger.Printf("%sRebooting\n", logPrefix)
		if *disableTriggers {
			return hadFailures
		}
		// If we get here, we are going to reboot and try harder if it fails.
		if logger, ok := logger.(flusher); ok {
			logger.Flush()
		}
		// Catch and log some signals to try and handle cases where the init
		// system signals subd but doesn't reboot, so we want to reach the hard
		// reboot fallback.
		signal.Reset(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		signals := make(chan os.Signal, 1)
		go handleSignals(signals, logger)
		signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		time.Sleep(time.Second)
		normalRebootAndWait(logger)
		time.Sleep(time.Second)
		forceRebootAndWait(logger)
		time.Sleep(time.Second)
		if err := osutil.HardReboot(logger); err != nil {
			logger.Printf("%sHard reboot failed: %s\n", logPrefix, err)
		}
		time.Sleep(time.Second)
		return true
	}
	if needRestart {
		logger.Printf("%sAction: service subd restart\n", logPrefix)
		if !osutil.RunCommand(logger, "service", "subd", "restart") {
			hadFailures = true
		}
	}
	return hadFailures
}
