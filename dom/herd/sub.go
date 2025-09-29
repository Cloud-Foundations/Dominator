package herd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	filegenclient "github.com/Cloud-Foundations/Dominator/lib/filegen/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/resourcepool"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	domproto "github.com/Cloud-Foundations/Dominator/proto/dominator"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/client"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

var (
	updateConfigurationsForSubs = flag.Bool("updateConfigurationsForSubs",
		true, "If true, update the configurations for all subs")
	logUnknownSubConnectErrors = flag.Bool("logUnknownSubConnectErrors", false,
		"If true, log unknown sub connection errors")
	showIP = flag.Bool("showIP", false,
		"If true, prefer to show IP address from MDB if available")
	useIP = flag.Bool("useIP", true,
		"If true, prefer to use IP address from MDB if available")

	subPortNumber = fmt.Sprintf(":%d", constants.SubPortNumber)
	zeroHash      hash.Hash
)

func (sub *Sub) string() string {
	if *showIP && sub.mdb.IpAddress != "" {
		return sub.mdb.IpAddress
	}
	return sub.mdb.Hostname
}

func (sub *Sub) address() string {
	if *useIP && sub.mdb.IpAddress != "" {
		hostInstance := strings.SplitN(sub.mdb.Hostname, "*", 2)
		if len(hostInstance) > 1 {
			return sub.mdb.IpAddress + "*" + hostInstance[1] + subPortNumber
		}
		return sub.mdb.IpAddress + subPortNumber
	}
	return sub.mdb.Hostname + subPortNumber
}

// Returns true if the principal described by authInfo has administrative access
// to the sub. It checks for method access, then ownership listed in the MDB
// data and then the sub configuration.
func (sub *Sub) checkAdminAccess(authInfo *srpc.AuthInformation) bool {
	if authInfo == nil {
		return false
	}
	if authInfo.HaveMethodAccess {
		return true
	}
	for _, group := range sub.mdb.OwnerGroups {
		if _, ok := authInfo.GroupList[group]; ok {
			return true
		}
	}
	if authInfo.Username != "" {
		for _, user := range sub.mdb.OwnerUsers {
			if user == authInfo.Username {
				return true
			}
		}
	}
	if sub.clientResource == nil {
		return false
	}
	srpcClient, err := sub.clientResource.GetHTTPWithDialer(sub.cancelChannel,
		sub.herd.dialer)
	if err != nil {
		return false
	}
	defer srpcClient.Put()
	conf, err := client.GetConfiguration(srpcClient)
	if err != nil {
		return false
	}
	for _, group := range conf.OwnerGroups {
		if _, ok := authInfo.GroupList[group]; ok {
			return true
		}
	}
	if authInfo.Username != "" {
		for _, user := range conf.OwnerUsers {
			if user == authInfo.Username {
				return true
			}
		}
	}
	return false
}

// TODO(rgooch): move this into the image manager.
func getComputedFiles(im *image.Image) []filegenclient.ComputedFile {
	if im == nil {
		return nil
	}
	numComputed := im.FileSystem.NumComputedRegularInodes()
	if numComputed < 1 {
		return nil
	}
	computedFiles := make([]filegenclient.ComputedFile, 0, numComputed)
	inodeToFilenamesTable := im.FileSystem.InodeToFilenamesTable()
	for inum, inode := range im.FileSystem.InodeTable {
		if inode, ok := inode.(*filesystem.ComputedRegularInode); ok {
			if filenames, ok := inodeToFilenamesTable[inum]; ok {
				if len(filenames) == 1 {
					computedFiles = append(computedFiles,
						filegenclient.ComputedFile{filenames[0], inode.Source})
				}
			}
		}
	}
	return computedFiles
}

// tryMakeBusy returns true if it made the sub busy, else false indicating that
// the sub is already busy. It does not block.
func (sub *Sub) tryMakeBusy() bool {
	sub.busyFlagMutex.Lock()
	defer sub.busyFlagMutex.Unlock()
	if sub.busy {
		return false
	}
	sub.busyStartTime = time.Now()
	sub.busy = true
	return true
}

// makeBusy waits until it makes the sub busy.
func (sub *Sub) makeBusy() {
	sleeper := backoffdelay.NewExponential(time.Millisecond, time.Second, 0)
	for {
		if sub.tryMakeBusy() {
			return
		}
		sleeper.Sleep()
	}
}

func (sub *Sub) makeUnbusy() {
	sub.busyFlagMutex.Lock()
	defer sub.busyFlagMutex.Unlock()
	sub.busyStopTime = time.Now()
	sub.busy = false
}

// Returns true if a client connection was open but the Poll failed due to an
// I/O error, indicating a retry is reasonable.
func (sub *Sub) connectAndPoll() bool {
	return sub.connectAndPoll2(false, false, nil)
}

// Returns true if a client connection was open but the Poll failed due to an
// I/O error, indicating a retry is reasonable.
// If swapImages is true, the required and planned images are swapped.
// If fastMessageChannel is not nil, fast updates are requested and relevant
// messages are sent to the channel.
func (sub *Sub) connectAndPoll2(swapImages, failOnReboot bool,
	fastMessageChannel chan<- FastUpdateMessage) bool {
	if sub.loadConfiguration(swapImages) {
		sub.generationCount = 0 // Force a full poll.
	}
	if sub.processFileUpdates() {
		sub.generationCount = 0 // Force a full poll.
	}
	sub.deletingFlagMutex.Lock()
	if sub.deleting {
		sub.deletingFlagMutex.Unlock()
		return false
	}
	if sub.clientResource == nil {
		sub.clientResource = srpc.NewClientResource("tcp", sub.address())
	}
	sub.deletingFlagMutex.Unlock()
	previousStatus := sub.status
	sub.status = statusConnecting
	timer := time.AfterFunc(time.Second, func() {
		sub.publishedStatus = sub.status
	})
	defer func() {
		timer.Stop()
		sub.publishedStatus = sub.status
		switch sub.status {
		case statusUnknown:
		case statusConnecting:
		case statusDNSError:
		case statusNoRouteToHost:
		case statusConnectionRefused,
			statusConnectTimeout,
			statusFailedToConnect:
			sub.herd.addSubToInstallerQueue(sub.mdb.Hostname)
		default:
			sub.herd.removeSubFromInstallerQueue(sub.mdb.Hostname)
		}
	}()
	sub.lastConnectionStartTime = time.Now()
	srpcClient, err := sub.clientResource.GetHTTPWithDialer(sub.cancelChannel,
		sub.herd.dialer)
	dialReturnedTime := time.Now()
	if err != nil {
		sub.isInsecure = false
		sub.pollTime = time.Time{}
		if err == resourcepool.ErrorResourceLimitExceeded {
			return false
		}
		if err, ok := err.(*net.OpError); ok {
			if _, ok := err.Err.(*net.DNSError); ok {
				sub.status = statusDNSError
				return false
			}
			if err.Timeout() {
				sub.status = statusConnectTimeout
				return false
			}
		}
		if err == srpc.ErrorConnectionRefused {
			sub.status = statusConnectionRefused
			return false
		}
		if err == srpc.ErrorNoRouteToHost {
			sub.status = statusNoRouteToHost
			return false
		}
		if err == srpc.ErrorMissingCertificate {
			sub.lastReachableTime = dialReturnedTime
			sub.status = statusMissingCertificate
			return false
		}
		if err == srpc.ErrorBadCertificate {
			sub.lastReachableTime = dialReturnedTime
			sub.status = statusBadCertificate
			return false
		}
		sub.status = statusFailedToConnect
		if *logUnknownSubConnectErrors {
			sub.herd.logger.Println(err)
		}
		return false
	}
	defer srpcClient.Put()
	if srpcClient.IsEncrypted() {
		sub.isInsecure = false
	} else {
		sub.isInsecure = true
	}
	sub.lastAddress = srpcClient.RemoteAddr()
	sub.lastReachableTime = dialReturnedTime
	sub.lastConnectionSucceededTime = dialReturnedTime
	sub.lastConnectDuration =
		sub.lastConnectionSucceededTime.Sub(sub.lastConnectionStartTime)
	connectDistribution.Add(sub.lastConnectDuration)
	waitStartTime := time.Now()
	sub.herd.cpuSharer.ReleaseCpu()
	select {
	case sub.herd.pollSemaphore <- struct{}{}:
		sub.herd.cpuSharer.GrabCpu()
		break
	case <-sub.cancelChannel:
		sub.herd.cpuSharer.GrabCpu()
		return false
	}
	pollWaitTimeDistribution.Add(time.Since(waitStartTime))
	if fastMessageChannel != nil {
		if err := client.BoostCpuLimit(srpcClient); err != nil {
			sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
		}
		sub.boostScanSpeed(srpcClient, fastMessageChannel)
	}
	sub.status = statusPolling
	retval := sub.poll(srpcClient, previousStatus, fastMessageChannel != nil,
		failOnReboot)
	<-sub.herd.pollSemaphore
	return retval
}

func (sub *Sub) boostScanSpeed(srpcClient *srpc.Client,
	fastMessageChannel chan<- FastUpdateMessage) {
	if fastMessageChannel == nil {
		return
	}
	if sub.configToRestore != nil {
		return
	}
	if err := client.BoostScanLimit(srpcClient); err == nil {
		return
	} else {
		sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
	}
	oldSubConfig, err := client.GetConfiguration(srpcClient)
	if err != nil {
		sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
		return
	}
	newSubConfig := oldSubConfig
	newSubConfig.NetworkSpeedPercent = 100
	newSubConfig.ScanSpeedPercent = 100
	if err := client.SetConfiguration(srpcClient, newSubConfig); err != nil {
		sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
		return
	}
	sub.configToRestore = &oldSubConfig
	sub.sendFastUpdateMessage(fastMessageChannel,
		"increased scan speed percent")
}

func (sub *Sub) restoreScanSpeed(fastMessageChannel chan<- FastUpdateMessage) {
	if fastMessageChannel == nil {
		return
	}
	if sub.configToRestore == nil {
		return
	}
	srpcClient, err := sub.clientResource.GetHTTPWithDialer(sub.cancelChannel,
		sub.herd.dialer)
	if err != nil {
		sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
		return
	}
	defer srpcClient.Put()
	err = client.SetConfiguration(srpcClient, *sub.configToRestore)
	if err != nil {
		sub.sendFastUpdateMessage(fastMessageChannel, err.Error())
		return
	}
	sub.sendFastUpdateMessage(fastMessageChannel,
		fmt.Sprintf("restored scan speed percent to %d%%",
			sub.configToRestore.ScanSpeedPercent))
	sub.configToRestore = nil
}

// getImageNames returns the required and planned image names from the MDB data.
// If swapImages is true, the image names are swapped.
func (sub *Sub) getImageNames(swapImages bool) (string, string) {
	if swapImages {
		return sub.mdb.PlannedImage, sub.mdb.RequiredImage
	} else {
		return sub.mdb.RequiredImage, sub.mdb.PlannedImage
	}
}

// Returns true if the images changed.
func (sub *Sub) loadConfiguration(swapImages bool) bool {
	// Get a stable copy of the configuration.
	requiredImageName, plannedImageName := sub.getImageNames(swapImages)
	if requiredImageName == "" {
		requiredImageName = sub.herd.defaultImageName
	}
	sub.herd.cpuSharer.ReleaseCpu()
	requiredImage := sub.herd.imageManager.GetNoError(requiredImageName)
	plannedImage := sub.herd.imageManager.GetNoError(plannedImageName)
	sub.herd.cpuSharer.GrabCpu()
	var changed bool
	if sub.requiredImage != requiredImage || sub.plannedImage != plannedImage {
		changed = true
	}
	sub.requiredImageName = requiredImageName
	sub.requiredImage = requiredImage
	sub.plannedImageName = plannedImageName
	sub.plannedImage = plannedImage
	return changed
}

func (sub *Sub) processFileUpdates() bool {
	haveUpdates := false
	image := sub.requiredImage
	if image != nil && sub.computedInodes == nil {
		sub.computedInodes = make(map[string]*filesystem.RegularInode)
		sub.deletingFlagMutex.Lock()
		if sub.deleting {
			sub.deletingFlagMutex.Unlock()
			return false
		}
		computedFiles := getComputedFiles(image)
		sub.herd.cpuSharer.ReleaseCpu()
		sub.herd.logger.Debugf(0,
			"processFileUpdates(%s): updating filegen manager\n", sub)
		sub.herd.computedFilesManager.Update(
			filegenclient.Machine{sub.mdb, computedFiles})
		sub.herd.cpuSharer.GrabCpu()
		sub.deletingFlagMutex.Unlock()
	}
	for _, fileInfos := range sub.fileUpdateReceiver.ReceiveAll() {
		if image == nil {
			for _, fileInfo := range fileInfos {
				sub.herd.logger.Printf(
					"processFileUpdates(%s): no image, discarding: %s: %x\n",
					sub, fileInfo.Pathname, fileInfo.Hash)
			}
			continue
		}
		filenameToInodeTable := image.FileSystem.FilenameToInodeTable()
		for _, fileInfo := range fileInfos {
			if fileInfo.Hash == zeroHash {
				continue // No object.
			}
			inum, ok := filenameToInodeTable[fileInfo.Pathname]
			if !ok {
				continue
			}
			genericInode, ok := image.FileSystem.InodeTable[inum]
			if !ok {
				continue
			}
			cInode, ok := genericInode.(*filesystem.ComputedRegularInode)
			if !ok {
				continue
			}
			rInode := &filesystem.RegularInode{
				Mode:         cInode.Mode,
				Uid:          cInode.Uid,
				Gid:          cInode.Gid,
				MtimeSeconds: -1, // The time is set during the compute.
				Size:         fileInfo.Length,
				Hash:         fileInfo.Hash,
			}
			sub.computedInodes[fileInfo.Pathname] = rInode
			haveUpdates = true
		}
	}
	return haveUpdates
}

// Returns true if the Poll failed due to an I/O error, indicating a retry is
// reasonable.
func (sub *Sub) poll(srpcClient *srpc.Client, previousStatus subStatus,
	fast, failOnReboot bool) bool {
	if err := srpcClient.SetTimeout(5 * time.Minute); err != nil {
		sub.herd.logger.Printf("poll(%s): error setting timeout: %s\n", sub)
	}
	// If the planned image has just become available, force a full poll.
	if previousStatus == statusSynced &&
		!sub.havePlannedImage &&
		sub.plannedImage != nil {
		sub.havePlannedImage = true
		sub.generationCount = 0 // Force a full poll.
	}
	// If the computed files have changed since the last sync, force a full poll
	if previousStatus == statusSynced &&
		sub.computedFilesChangeTime.After(sub.lastSyncTime) {
		sub.generationCount = 0 // Force a full poll.
	}
	// If the last update was disabled and updates are enabled now, force a full
	// poll.
	if previousStatus == statusUpdatesDisabled &&
		sub.herd.updatesDisabledReason == "" && !sub.mdb.DisableUpdates {
		sub.generationCount = 0 // Force a full poll.
	}
	// If the last update was disabled due to a safety check and there is a
	// pending SafetyClear, force a full poll to re-compute the update.
	if previousStatus == statusUnsafeUpdate && sub.pendingSafetyClear {
		sub.generationCount = 0 // Force a full poll.
	}
	// If the last update failed because disruption was not permitted and there
	// is a pending ForceDisruption, force a full poll to re-compute the update.
	if (previousStatus == statusDisruptionRequested ||
		previousStatus == statusDisruptionDenied) &&
		sub.pendingForceDisruptiveUpdate {
		sub.generationCount = 0 // Force a full poll.
	}
	var request subproto.PollRequest
	request.HaveGeneration = sub.generationCount
	var reply subproto.PollResponse
	haveImage := false
	if sub.requiredImage == nil && sub.plannedImage == nil {
		request.ShortPollOnly = true
		// Ensure a full poll when the image becomes available later. This will
		// cover the special case when an image expiration is extended, which
		// leads to the sub showing "image not ready" until the next generation
		// increment.
		sub.generationCount = 0
	} else {
		haveImage = true
	}
	logger := sub.herd.logger
	sub.lastPollStartTime = time.Now()
	if err := client.CallPoll(srpcClient, request, &reply); err != nil {
		srpcClient.Close()
		if err == io.EOF {
			return true
		}
		sub.pollTime = time.Time{}
		var retval bool
		if err == srpc.ErrorAccessToMethodDenied {
			sub.status = statusPollDenied
		} else {
			sub.status = statusFailedToPoll
			retval = true
		}
		logger.Printf("Error calling %s.Poll(%s): %s\n",
			sub, format.Duration(time.Since(sub.lastPollStartTime)), err)
		return retval
	}
	sub.lastDisruptionState = reply.DisruptionState
	sub.lastPollSucceededTime = time.Now()
	sub.lastSuccessfulImageName = reply.LastSuccessfulImageName
	sub.lastNote = reply.LastNote
	sub.lastWriteError = reply.LastWriteError
	sub.systemUptime = reply.SystemUptime
	if reply.GenerationCount == 0 {
		sub.reclaim()
		sub.generationCount = 0
	}
	sub.lastScanDuration = reply.DurationOfLastScan
	if fs := reply.FileSystem; fs == nil {
		sub.lastPollWasFull = false
		sub.lastShortPollDuration =
			sub.lastPollSucceededTime.Sub(sub.lastPollStartTime)
		shortPollDistribution.Add(sub.lastShortPollDuration)
		if !sub.startTime.Equal(reply.StartTime) {
			sub.generationCount = 0 // Sub has restarted: force a full poll.
		}
		if sub.freeSpaceThreshold != nil && reply.FreeSpace != nil {
			if *reply.FreeSpace > *sub.freeSpaceThreshold {
				sub.generationCount = 0 // Force a full poll for next time.
			}
		}
	} else {
		sub.lastPollWasFull = true
		sub.freeSpaceThreshold = nil
		if err := fs.RebuildInodePointers(); err != nil {
			sub.status = statusFailedToPoll
			logger.Printf("Error building pointers for: %s %s\n", sub, err)
			return false
		}
		fs.BuildEntryMap()
		sub.fileSystem = fs
		sub.objectCache = reply.ObjectCache
		sub.generationCount = reply.GenerationCount
		sub.lastFullPollDuration =
			sub.lastPollSucceededTime.Sub(sub.lastPollStartTime)
		fullPollDistribution.Add(sub.lastFullPollDuration)
	}
	sub.startTime = reply.StartTime
	sub.pollTime = reply.PollTime
	sub.updateConfiguration(srpcClient, reply)
	if reply.FetchInProgress {
		sub.status = statusFetching
		return false
	}
	if reply.UpdateInProgress {
		sub.status = statusUpdating
		return false
	}
	if reply.LastWriteError != "" {
		sub.status = statusUnwritable
		sub.reclaim()
		return false
	}
	if reply.GenerationCount < 1 {
		sub.status = statusSubNotReady
		return false
	}
	if reply.LockedByAnotherClient {
		sub.status = statusLocked
		sub.reclaim()
		return false
	}
	if previousStatus == statusLocked { // Not locked anymore, but was locked.
		if sub.fileSystem == nil {
			sub.generationCount = 0 // Force a full poll next cycle.
			return false
		}
	}
	if previousStatus == statusFetching && reply.LastFetchError != "" {
		logger.Printf("Fetch failure for: %s: %s\n", sub, reply.LastFetchError)
		sub.status = statusFailedToFetch
		if sub.fileSystem == nil {
			sub.generationCount = 0 // Force a full poll next cycle.
			return false
		}
	}
	if previousStatus == statusUpdating {
		// Transition from updating to update ended (may be partial/failed).
		switch reply.LastUpdateError {
		case "":
			sub.status = statusWaitingForNextFullPoll
		case subproto.ErrorDisruptionPending:
			sub.status = statusDisruptionRequested
		case subproto.ErrorDisruptionDenied:
			sub.status = statusDisruptionDenied
		default:
			logger.Printf("Update failure for: %s: %s\n",
				sub, reply.LastUpdateError)
			sub.status = statusFailedToUpdate
		}
		sub.scanCountAtLastUpdateEnd = reply.ScanCount
		sub.reclaim()
		return false
	}
	if sub.checkCancel() {
		// Configuration change pending: skip further processing. Do not reclaim
		// file-system and objectcache data: it will speed up the next Poll.
		return false
	}
	if !haveImage {
		if sub.requiredImageName == "" {
			sub.status = statusImageUndefined
		} else {
			sub.status = statusImageNotReady
		}
		return false
	}
	if previousStatus == statusFailedToUpdate ||
		previousStatus == statusWaitingForNextFullPoll {
		if sub.scanCountAtLastUpdateEnd == reply.ScanCount {
			// Need to wait until sub has performed a new scan.
			if sub.fileSystem != nil {
				sub.reclaim()
			}
			sub.status = previousStatus
			return false
		}
		if sub.fileSystem == nil {
			// Force a full poll next cycle so that we can see the state of the
			// sub.
			sub.generationCount = 0
			sub.status = previousStatus
			return false
		}
	}
	if previousStatus == statusDisruptionRequested ||
		previousStatus == statusDisruptionDenied {
		switch reply.DisruptionState {
		case subproto.DisruptionStateAnytime:
			sub.generationCount = 0
		case subproto.DisruptionStatePermitted:
			sub.generationCount = 0
		case subproto.DisruptionStateRequested:
			previousStatus = statusDisruptionRequested
		case subproto.DisruptionStateDenied:
			previousStatus = statusDisruptionDenied
		}
	}
	if sub.fileSystem == nil {
		sub.status = previousStatus
		return false
	}
	if sub.requiredImage != nil {
		idle, status := sub.fetchMissingObjects(srpcClient, sub.requiredImage,
			reply.FreeSpace, true, fast)
		if !idle {
			sub.status = status
			sub.reclaim()
			return false
		}
		sub.status = statusComputingUpdate
		if idle, status := sub.sendUpdate(srpcClient, failOnReboot); !idle {
			sub.status = status
			sub.reclaim()
			return false
		}
	} else {
		sub.status = statusImageNotReady
	}
	if sub.plannedImage != sub.requiredImage {
		idle, status := sub.fetchMissingObjects(srpcClient, sub.plannedImage,
			reply.FreeSpace, false, fast)
		if !idle {
			if status != statusImageNotReady &&
				status != statusNotEnoughFreeSpace {
				sub.status = status
				sub.reclaim()
				return false
			}
		}
	}
	if previousStatus == statusWaitingForNextFullPoll &&
		!sub.lastUpdateTime.IsZero() {
		sub.lastSyncTime = time.Now()
	}
	sub.status = statusSynced
	sub.cleanup(srpcClient)
	sub.reclaim()
	return false
}

func (sub *Sub) reclaim() {
	sub.fileSystem = nil  // Mark memory for reclaim.
	sub.objectCache = nil // Mark memory for reclaim.
}

func (sub *Sub) updateConfiguration(srpcClient *srpc.Client,
	pollReply subproto.PollResponse) {
	if !*updateConfigurationsForSubs {
		return
	}
	if pollReply.ScanCount < 1 {
		return
	}
	sub.herd.RLockWithTimeout(time.Minute)
	newConf := sub.herd.configurationForSubs
	sub.herd.RUnlock()
	if newConf.CpuPercent < 1 {
		newConf.CpuPercent = pollReply.CurrentConfiguration.CpuPercent
	}
	if newConf.NetworkSpeedPercent < 1 {
		newConf.NetworkSpeedPercent =
			pollReply.CurrentConfiguration.NetworkSpeedPercent
	}
	if newConf.ScanSpeedPercent < 1 {
		newConf.ScanSpeedPercent =
			pollReply.CurrentConfiguration.ScanSpeedPercent
	}
	if compareConfigs(pollReply.CurrentConfiguration, newConf) {
		return
	}
	if err := client.SetConfiguration(srpcClient, newConf); err != nil {
		srpcClient.Close()
		logger := sub.herd.logger
		logger.Printf("Error setting configuration for sub: %s: %s\n",
			sub, err)
		return
	}
}

func compareConfigs(oldConf, newConf subproto.Configuration) bool {
	if newConf.CpuPercent != oldConf.CpuPercent {
		return false
	}
	if newConf.NetworkSpeedPercent != oldConf.NetworkSpeedPercent {
		return false
	}
	if newConf.ScanSpeedPercent != oldConf.ScanSpeedPercent {
		return false
	}
	if len(newConf.ScanExclusionList) != len(oldConf.ScanExclusionList) {
		return false
	}
	for index, newString := range newConf.ScanExclusionList {
		if newString != oldConf.ScanExclusionList[index] {
			return false
		}
	}
	return true
}

// Returns true if all required objects are available.
func (sub *Sub) fetchMissingObjects(srpcClient *srpc.Client, img *image.Image,
	freeSpace *uint64, isRequiredImage, fast bool) (
	bool, subStatus) {
	if img == nil {
		return false, statusImageNotReady
	}
	var imageType string
	if isRequiredImage {
		imageType = "required"
	} else {
		imageType = "planned"
	}
	logger := sub.herd.logger
	subObj := lib.Sub{
		Hostname:       sub.mdb.Hostname,
		Client:         srpcClient,
		FileSystem:     sub.fileSystem,
		ComputedInodes: sub.computedInodes,
		ObjectCache:    sub.objectCache,
		ObjectGetter:   sub.herd.objectServer}
	startTime := time.Now()
	objectsToFetch, objectsToPush := lib.BuildMissingLists(subObj, img,
		isRequiredImage, false, logger)
	sub.herd.logger.Debugf(0,
		"lib.BuildMissingLists(%s) for %s image took: %s\n",
		sub, imageType, format.Duration(time.Since(startTime)))
	if objectsToPush == nil {
		return false, statusMissingComputedFile
	}
	var returnAvailable bool = true
	var returnStatus subStatus = statusSynced
	if len(objectsToFetch) > 0 {
		if !sub.checkForEnoughSpace(freeSpace, objectsToFetch) {
			return false, statusNotEnoughFreeSpace
		}
		logger.Printf("Calling %s:Subd.Fetch(%s) for: %d objects\n",
			sub, imageType, len(objectsToFetch))
		request := subproto.FetchRequest{
			ServerAddress: sub.herd.imageManager.String(),
			Hashes:        objectcache.ObjectMapToCache(objectsToFetch),
		}
		if fast {
			request.SpeedPercent = 100
		}
		var response subproto.FetchResponse
		err := client.CallFetch(srpcClient, request, &response)
		if err != nil {
			srpcClient.Close()
			logger.Printf("Error calling %s:Subd.Fetch(): %s\n", sub, err)
			if err == srpc.ErrorAccessToMethodDenied {
				return false, statusFetchDenied
			}
			return false, statusFailedToFetch
		}
		returnAvailable = false
		returnStatus = statusFetching
	}
	if len(objectsToPush) > 0 {
		logger.Printf("Calling %s:ObjectServer.AddObjects() for: %d objects\n",
			sub, len(objectsToPush))
		sub.herd.cpuSharer.GrabSemaphore(sub.herd.pushSemaphore)
		defer func() { <-sub.herd.pushSemaphore }()
		sub.status = statusPushing
		err := lib.PushObjects(subObj, objectsToPush, logger)
		if err != nil {
			if err == srpc.ErrorAccessToMethodDenied {
				return false, statusPushDenied
			}
			if err == lib.ErrorFailedToGetObject {
				return false, statusFailedToGetObject
			}
			return false, statusFailedToPush
		}
		if returnAvailable {
			// Update local copy of objectcache, since there will not be
			// another Poll() before the update computation.
			for hashVal := range objectsToPush {
				sub.objectCache = append(sub.objectCache, hashVal)
			}
		}
	}
	return returnAvailable, returnStatus
}

// Returns true if no update needs to be performed.
func (sub *Sub) sendUpdate(srpcClient *srpc.Client,
	failOnReboot bool) (bool, subStatus) {
	logger := sub.herd.logger
	var request subproto.UpdateRequest
	var reply subproto.UpdateResponse
	if idle, missing := sub.buildUpdateRequest(&request); missing {
		return false, statusMissingComputedFile
	} else if idle {
		return true, statusSynced
	}
	if sub.mdb.DisableUpdates || sub.herd.updatesDisabledReason != "" {
		return false, statusUpdatesDisabled
	}
	if !sub.pendingSafetyClear {
		// Perform a cheap safety check: if over half the inodes will be deleted
		// then mark the update as unsafe.
		if sub.checkForUnsafeChange(request) {
			return false, statusUnsafeUpdate
		}
	}
	if failOnReboot {
		triggers := sublib.MatchTriggersInUpdate(request)
		_, reboot := sublib.CheckImpact(triggers)
		if reboot {
			return false, statusRebootBlocked
		}
	}
	if value, ok := sub.mdb.Tags["ForceDisruptiveUpdate"]; ok {
		if strings.EqualFold(value, "true") {
			request.ForceDisruption = true
		}
	}
	if sub.pendingForceDisruptiveUpdate {
		request.ForceDisruption = true
	}
	sub.status = statusSendingUpdate
	sub.lastUpdateTime = time.Now()
	logger.Printf("Calling %s:Subd.Update() for image: %s\n",
		sub, sub.requiredImageName)
	if err := client.CallUpdate(srpcClient, request, &reply); err != nil {
		srpcClient.Close()
		logger.Printf("Error calling %s:Subd.Update(): %s\n", sub, err)
		if err == srpc.ErrorAccessToMethodDenied {
			return false, statusUpdateDenied
		}
		return false, statusFailedToUpdate
	}
	sub.pendingSafetyClear = false
	sub.pendingForceDisruptiveUpdate = false
	return false, statusUpdating
}

// Returns true if the change is unsafe (very large number of deletions).
func (sub *Sub) checkForUnsafeChange(request subproto.UpdateRequest) bool {
	if sub.requiredImage.Filter == nil {
		return false // Sparse image: no deletions.
	}
	if _, ok := sub.mdb.Tags["DisableSafetyCheck"]; ok {
		return false // This sub doesn't need a safety check.
	}
	if len(sub.requiredImage.FileSystem.InodeTable) <
		len(sub.fileSystem.InodeTable)>>1 {
		return true
	}
	if len(request.PathsToDelete) > len(sub.fileSystem.InodeTable)>>1 {
		return true
	}
	return false
}

// cleanup will tell the Sub to remove unused objects and that any disruptive
// updates have completed.
func (sub *Sub) cleanup(srpcClient *srpc.Client) {
	startTime := time.Now()
	logger := sub.herd.logger
	unusedObjects := make(map[hash.Hash]bool)
	for _, hash := range sub.objectCache {
		unusedObjects[hash] = false // Potential cleanup candidate.
	}
	for _, inode := range sub.fileSystem.InodeTable {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				if _, ok := unusedObjects[inode.Hash]; ok {
					unusedObjects[inode.Hash] = true // Must clean this one up.
				}
			}
		}
	}
	image := sub.plannedImage
	if image != nil {
		for _, inode := range image.FileSystem.InodeTable {
			if inode, ok := inode.(*filesystem.RegularInode); ok {
				if inode.Size > 0 {
					if clean, ok := unusedObjects[inode.Hash]; !clean && ok {
						delete(unusedObjects, inode.Hash)
					}
				}
			}
		}
	}
	if len(unusedObjects) < 1 &&
		sub.lastDisruptionState == subproto.DisruptionStateAnytime {
		cleanupComputeTimeDistribution.Add(time.Since(startTime))
		return
	}
	hashes := make([]hash.Hash, 0, len(unusedObjects))
	for hash := range unusedObjects {
		hashes = append(hashes, hash)
	}
	cleanupComputeTimeDistribution.Add(time.Since(startTime))
	startTime = time.Now()
	if err := client.Cleanup(srpcClient, hashes); err != nil {
		srpcClient.Close()
		logger.Printf("Error calling %s:Subd.Cleanup(): %s\n", sub, err)
	}
	cleanupTimeDistribution.Add(time.Since(startTime))
}

func (sub *Sub) checkForEnoughSpace(freeSpace *uint64,
	objects map[hash.Hash]uint64) bool {
	if freeSpace == nil {
		sub.freeSpaceThreshold = nil
		return true // Don't know, assume OK.
	}
	var totalUsage uint64
	for _, size := range objects {
		usage := (size >> 12) << 12
		if usage < size {
			usage += 1 << 12
		}
		totalUsage += usage
	}
	if *freeSpace > totalUsage {
		sub.freeSpaceThreshold = nil
		return true
	}
	sub.freeSpaceThreshold = &totalUsage
	return false
}

func (sub *Sub) clearSafetyShutoff(authInfo *srpc.AuthInformation) error {
	if sub.status != statusUnsafeUpdate {
		return errors.New("no pending unsafe update")
	}
	if !sub.checkAdminAccess(authInfo) {
		return errors.New("no access to sub")
	}
	sub.pendingSafetyClear = true
	return nil
}

func (sub *Sub) checkCancel() bool {
	select {
	case <-sub.cancelChannel:
		return true
	default:
		return false
	}
}

func (sub *Sub) fastUpdate(request domproto.FastUpdateRequest,
	authInfo *srpc.AuthInformation) (
	<-chan FastUpdateMessage, error) {
	if !sub.checkAdminAccess(authInfo) {
		return nil, errors.New("no access to sub")
	}
	if request.UsePlannedImage {
		if sub.plannedImageName == "" {
			return nil, errors.New("no PlannedImage specified")
		}
		if sub.plannedImage == nil {
			return nil, fmt.Errorf("image: %s does not exist yet",
				sub.plannedImageName)
		}
	} else {
		if sub.requiredImageName == "" {
			return nil, errors.New("no RequiredImage specified")
		}
		if sub.requiredImage == nil {
			return nil, fmt.Errorf("image: %s does not exist yet",
				sub.requiredImageName)
		}
	}
	progressChannel := make(chan FastUpdateMessage, 16)
	go sub.processFastUpdate(progressChannel, request)
	return progressChannel, nil
}

func (sub *Sub) forceDisruptiveUpdate(authInfo *srpc.AuthInformation) error {
	switch sub.status {
	case statusDisruptionRequested:
	case statusDisruptionDenied:
	default:
		return errors.New("not waiting for disruptive update permission")
	}
	if !sub.checkAdminAccess(authInfo) {
		return errors.New("no access to sub")
	}
	sub.pendingForceDisruptiveUpdate = true
	return nil
}

func (sub *Sub) sendCancel() {
	select {
	case sub.cancelChannel <- struct{}{}:
	default:
	}
}

func (sub *Sub) processFastUpdate(progressChannel chan<- FastUpdateMessage,
	request domproto.FastUpdateRequest) {
	defer close(progressChannel)
	select {
	case sub.herd.fastUpdateSemaphore <- struct{}{}:
		sub.sendFastUpdateMessage(progressChannel, "got fast update slot")
	default:
		sub.sendFastUpdateMessage(progressChannel,
			"waiting for fast update slot")
		sub.herd.fastUpdateSemaphore <- struct{}{}
		sub.sendFastUpdateMessage(progressChannel,
			"finished waiting for fast update slot")
	}
	defer func() {
		<-sub.herd.fastUpdateSemaphore
	}()
	if !sub.tryMakeBusy() {
		sub.sendFastUpdateMessage(progressChannel,
			"waiting for sub to not be busy")
		sub.makeBusy()
	}
	defer sub.makeUnbusy()
	sub.sendFastUpdateMessage(progressChannel, "made sub busy")
	sub.herd.cpuSharer.GrabCpu()
	defer sub.herd.cpuSharer.ReleaseCpu()
	if request.UsePlannedImage && sub.plannedImage != nil {
		sub.herd.computedFilesManager.Update(
			filegenclient.Machine{sub.mdb, getComputedFiles(sub.plannedImage)})
	}
	origRequiredImage := sub.requiredImage
	defer func() {
		if request.UsePlannedImage && origRequiredImage != nil {
			sub.herd.computedFilesManager.Update(
				filegenclient.Machine{sub.mdb,
					getComputedFiles(origRequiredImage)})
		}
		sub.pendingForceDisruptiveUpdate = false
		sub.pendingSafetyClear = false
	}()
	sleeper := backoffdelay.NewExponential(10*time.Millisecond, time.Second, 2)
	sleeper.SetSleepFunc(sub.herd.cpuSharer.Sleep)
	var prevStatus subStatus
	timeoutTime := time.Now().Add(request.Timeout)
	defer sub.restoreScanSpeed(progressChannel)
	if sub.status == statusSynced {
		sub.status = statusWaitingToPoll
	}
	for ; time.Until(timeoutTime) > 0; sleeper.Sleep() {
		if sub.deleting {
			sub.sendFastUpdateMessage(progressChannel, "deleting")
			return
		}
		if sub.status != prevStatus {
			sub.sendFastUpdateMessage(progressChannel, sub.status.String())
			prevStatus = sub.status
			sleeper.Reset()
		}
		switch sub.status {
		case statusSynced,
			statusUpdatesDisabled,
			statusUnsafeUpdate,
			statusRebootBlocked:
			return
		default:
		}
		sub.pendingForceDisruptiveUpdate = request.ForceDisruptiveUpdate
		sub.pendingSafetyClear = request.DisableSafetyCheck
		sub.connectAndPoll2(request.UsePlannedImage, request.FailOnReboot,
			progressChannel)
	}
	sub.sendFastUpdateMessage(progressChannel, "timed out")
}

func (sub *Sub) sendFastUpdateMessage(ch chan<- FastUpdateMessage,
	message string) {
	ch <- FastUpdateMessage{
		Message: message,
		Synced:  sub.status == statusSynced,
	}
}
