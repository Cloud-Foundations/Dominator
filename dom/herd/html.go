package herd

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

var timeFormat string = "02 Jan 2006 15:04:05.99 MST"

type rDuration time.Duration

func (d rDuration) selector(sub *Sub) bool {
	if time.Since(sub.lastReachableTime) <= time.Duration(d) {
		return true
	}
	return false
}

func (herd *Herd) writeHtml(writer io.Writer) {
	if herd.updatesDisabledReason != "" {
		herd.writeDisableStatus(writer)
		fmt.Fprintln(writer, "<br>")
	}
	var numAliveSubs, numCompliantSubs, numDeviantSubs uint64
	var numLikelyCompliantSubs, numDisruptionWaitingSubs uint64
	var reachableMinuteSubs, reachable10MinuteSubs, reachableHourSubs uint64
	var reachableDaySubs, reachableWeekSubs, reachableMonthSubs uint64
	subCounters := []subCounter{
		{&numAliveSubs, selectAliveSub},
		{&numCompliantSubs, selectCompliantSub},
		{&numDeviantSubs, selectDeviantSub},
		{&numLikelyCompliantSubs, selectLikelyCompliantSub},
		{&numDisruptionWaitingSubs, selectDisruptionWaitingSub},
		{&reachableMinuteSubs, rDuration(time.Minute).selector},
		{&reachable10MinuteSubs, rDuration(10 * time.Minute).selector},
		{&reachableHourSubs, rDuration(time.Hour).selector},
		{&reachableDaySubs, rDuration(24 * time.Hour).selector},
		{&reachableWeekSubs, rDuration(7 * 24 * time.Hour).selector},
		{&reachableMonthSubs, rDuration(730 * time.Hour).selector},
	}
	numSubs := herd.countSelectedSubs(subCounters)
	fmt.Fprintf(writer, "Time since current cycle start: %s<br>\n",
		time.Since(herd.currentScanStartTime))
	if numSubs < 1 {
		fmt.Fprintf(writer, "Duration of previous cycle: %s<br>\n",
			herd.previousScanDuration)
	} else {
		fmt.Fprintf(writer, "Duration of previous cycle: %s (%s/sub)<br>\n",
			herd.previousScanDuration,
			herd.previousScanDuration/time.Duration(numSubs))
	}
	fmt.Fprintf(writer, "Average cycle time: %s<br>\n",
		format.Duration(herd.totalScanDuration/time.Duration(herd.scanCounter)))
	fmt.Fprintf(writer, "Cycle count: %d<br>\n", herd.scanCounter)
	fmt.Fprintf(writer, "Image server: <a href=\"http://%s/\">%s</a><br>\n",
		herd.imageManager, herd.imageManager)
	if herd.defaultImageName != "" {
		fmt.Fprintf(writer,
			"Default image: <a href=\"http://%s/showImage?%s\">%s</a><br>\n",
			herd.imageManager, herd.defaultImageName, herd.defaultImageName)
	}
	fmt.Fprintf(writer,
		"Number of <a href=\"listSubs\">subs</a>: <a href=\"showAllSubs\">%d</a>",
		numSubs)
	fmt.Fprintf(writer,
		" (<a href=\"showAllSubs?output=json\">JSON</a>")
	fmt.Fprintf(writer,
		", <a href=\"showAllSubs?output=csv\">CSV</a>)<br>\n")
	fmt.Fprintf(writer,
		"Number of alive subs: <a href=\"showAliveSubs\">%d</a>",
		numAliveSubs)
	fmt.Fprintf(writer,
		" (<a href=\"showAliveSubs?output=json\">JSON</a>")
	fmt.Fprintf(writer,
		", <a href=\"showAliveSubs?output=csv\">CSV</a>)<br>\n")
	fmt.Fprint(writer, "Number of reachable subs in last: ")
	herd.writeReachableSubsLink(writer, reachableMinuteSubs, "1 min", "1m",
		true)
	herd.writeReachableSubsLink(writer, reachable10MinuteSubs, "10 min", "10m",
		true)
	herd.writeReachableSubsLink(writer, reachableHourSubs, "1 hour", "1h", true)
	herd.writeReachableSubsLink(writer, reachableDaySubs, "1 day", "1d", true)
	herd.writeReachableSubsLink(writer, reachableWeekSubs, "1 week", "1w",
		true)
	herd.writeReachableSubsLink(writer, reachableMonthSubs, "1 month", "1M",
		false)
	fmt.Fprintf(writer,
		"Number of deviant subs: <a href=\"showDeviantSubs\">%d</a>",
		numDeviantSubs)
	fmt.Fprintf(writer,
		" (<a href=\"showDeviantSubs?output=json\">JSON</a>")
	fmt.Fprintf(writer,
		", <a href=\"showDeviantSubs?output=csv\">CSV</a>)<br>\n")
	fmt.Fprintf(writer,
		"Number of compliant subs: <a href=\"showCompliantSubs\">%d</a>(verified)",
		numCompliantSubs)
	fmt.Fprintf(writer,
		", <a href=\"showLikelyCompliantSubs\">%d</a>(likely)<br>\n",
		numLikelyCompliantSubs)
	if numDisruptionWaitingSubs > 0 {
		fmt.Fprintf(writer,
			"Number of subs waiting to disrupt: <a href=\"showAllSubs?status=disruption%%20requested&status=disruption%%20denied\">%d</a>",
			numDisruptionWaitingSubs)
		fmt.Fprintf(writer,
			" (<a href=\"showAllSubs?status=disruption%%20requested&status=disruption%%20denied&output=json\">JSON</a>")
		fmt.Fprintf(writer,
			", <a href=\"showAllSubs?status=disruption%%20requested&status=disruption%%20denied&output=csv\">CSV</a>)<br>\n")
	}
	fmt.Fprintf(writer,
		"Image status for subs: <a href=\"showImagesForSubs\">dashboard</a>")
	fmt.Fprintf(writer,
		" (<a href=\"listImagesForSubs?output=json\">JSON</a>")
	fmt.Fprintf(writer,
		", <a href=\"listImagesForSubs?output=csv\">CSV</a>)<br>\n")
	subs := herd.getSelectedSubs(nil)
	connectDurations := getConnectDurations(subs)
	shortPollDurations := getPollDurations(subs, false)
	fullPollDurations := getPollDurations(subs, true)
	showDurationStats(writer, connectDurations, "Connect")
	showDurationStats(writer, shortPollDurations, "Short poll")
	showDurationStats(writer, fullPollDurations, "Full poll")
	// TODO(rgooch): Figure out a way of restoring this information.
	//fmt.Fprintf(writer, "Connection slots: %d out of %d<br>\n",
	//len(herd.connectionSemaphore), cap(herd.connectionSemaphore))
	fmt.Fprintf(writer, "Poll slots: %d out of %d<br>\n",
		len(herd.pollSemaphore), cap(herd.pollSemaphore))
	stats := herd.cpuSharer.GetStatistics()
	timeSinceLastIdleEvent := time.Since(stats.LastIdleEvent)
	fmt.Fprintf(writer,
		"CPU slots: %d out of %d; idle events: %d, last: %s, time since: %s, last acquire: %s, last yield: %s<br>\n",
		stats.NumCpuRunning, stats.NumCpu, stats.NumIdleEvents,
		stats.LastIdleEvent.Format(timeFormat),
		format.Duration(timeSinceLastIdleEvent),
		format.Duration(time.Since(stats.LastAcquireEvent)),
		format.Duration(time.Since(stats.LastYieldEvent)))
}

func (herd *Herd) writeDisableStatus(writer io.Writer) {
	timeString := ""
	if !herd.updatesDisabledTime.IsZero() {
		timeString = " at " + herd.updatesDisabledTime.Format(timeFormat)
	}
	if herd.updatesDisabledBy == "" {
		fmt.Fprintf(writer,
			"<font color=\"red\">Updates disabled %s</font>%s\n",
			herd.updatesDisabledReason, timeString)
	} else {
		fmt.Fprintf(writer,
			"<font color=\"red\">Updates disabled by: %s %s</font>%s",
			herd.updatesDisabledBy, herd.updatesDisabledReason, timeString)
	}
}

func (herd *Herd) writeReachableSubsLink(writer io.Writer,
	numSubs uint64, durationString string, query string, moreToCome bool) {
	fmt.Fprintf(writer, "<a href=\"showReachableSubs?last=%s\">%s</a>",
		query, durationString)
	fmt.Fprintf(writer, "(<a href=\"listReachableSubs?last=%s\">%d</a>)",
		query, numSubs)
	if moreToCome {
		fmt.Fprint(writer, ", ")
	} else {
		fmt.Fprintln(writer, "<br>")
	}
}

func selectAliveSub(sub *Sub) bool {
	switch sub.publishedStatus {
	case statusUnknown:
		return false
	case statusConnecting:
		return false
	case statusDNSError:
		return false
	case statusConnectionRefused:
		return false
	case statusNoRouteToHost:
		return false
	case statusConnectTimeout:
		return false
	case statusFailedToConnect:
		return false
	case statusFailedToPoll:
		return false
	}
	return true
}

func selectDeviantSub(sub *Sub) bool {
	switch sub.publishedStatus {
	case statusWaitingToPoll:
		return true
	case statusNotEnoughFreeSpace:
		return true
	case statusFetching, statusFetchDenied, statusFailedToFetch:
		return true
	case statusPushing, statusPushDenied, statusFailedToPush:
		return true
	case statusFailedToGetObject:
		return true
	case statusComputingUpdate:
		return sub.lastSuccessfulImageName != sub.mdb.RequiredImage
	case statusSendingUpdate:
		return true
	case statusMissingComputedFile:
		return true
	case statusUpdatesDisabled:
		return true
	case statusUpdating:
		return true
	case statusUpdateDenied:
		return true
	case statusFailedToUpdate:
		return true
	}
	return false
}

func selectCompliantSub(sub *Sub) bool {
	if sub.publishedStatus == statusSynced {
		return true
	}
	return false
}

func selectDisruptionWaitingSub(sub *Sub) bool {
	switch sub.publishedStatus {
	case statusDisruptionRequested:
		return true
	case statusDisruptionDenied:
		return true
	}
	return false
}

func selectLikelyCompliantSub(sub *Sub) bool {
	switch sub.publishedStatus {
	case statusWaitingToPoll, statusPolling:
		return sub.lastSuccessfulImageName == sub.mdb.RequiredImage
	case statusWaitingForNextFullPoll:
		return true
	case statusSynced:
		return true
	}
	return false
}

func getConnectDurations(subs []*Sub) []time.Duration {
	durations := make([]time.Duration, 0, len(subs))
	for _, sub := range subs {
		if sub.lastConnectDuration > 0 {
			durations = append(durations, sub.lastConnectDuration)
		}
	}
	sort.Sort(durationList(durations))
	return durations
}

func getPollDurations(subs []*Sub, full bool) []time.Duration {
	durations := make([]time.Duration, 0, len(subs))
	for _, sub := range subs {
		var duration time.Duration
		if full {
			duration = sub.lastFullPollDuration
		} else {
			duration = sub.lastShortPollDuration
		}
		if duration > 0 {
			durations = append(durations, duration)
		}
	}
	sort.Sort(durationList(durations))
	return durations
}

type durationList []time.Duration

func (dl durationList) Len() int {
	return len(dl)
}

func (dl durationList) Less(i, j int) bool {
	return dl[i] < dl[j]
}

func (dl durationList) Swap(i, j int) {
	dl[i], dl[j] = dl[j], dl[i]
}

func showDurationStats(writer io.Writer, durations []time.Duration,
	durationType string) {
	if len(durations) < 1 {
		return
	}
	var avgDuration time.Duration
	for _, duration := range durations {
		avgDuration += duration
	}
	avgDuration /= time.Duration(len(durations))
	medDuration := durations[len(durations)/2]
	unit := "ns"
	scale := 1.0
	switch {
	case medDuration > 1e9:
		unit = "s"
		scale = 1e-9
	case medDuration > 1e6:
		unit = "ms"
		scale = 1e-6
	case medDuration > 1e3:
		unit = "µs"
		scale = 1e-3
	}
	fmt.Fprintf(writer,
		"%s durations: %.3f/%.3f/%.3f/%.3f %s (avg/med/min/max)<br>\n",
		durationType,
		float64(avgDuration)*scale, float64(medDuration)*scale,
		float64(durations[0])*scale, float64(durations[len(durations)-1])*scale,
		unit)
}
