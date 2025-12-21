package main

import (
	"fmt"
	"os"
	"time"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	domproto "github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func fastUpdateSubcommand(args []string, logger log.DebugLogger) error {
	rebootBlocked, err := fastUpdate(getClient(), args[0], logger)
	if err == nil {
		return nil
	}
	err = fmt.Errorf("error doing fast update: %s", err)
	if rebootBlocked {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	return err
}

func fastUpdate(client *srpc.Client, subHostname string,
	logger log.DebugLogger) (bool, error) {
	startTime := time.Now()
	logger = prefixlogger.New(subHostname+": ", logger)
	reply, err := domclient.FastUpdateDetailed(client,
		domproto.FastUpdateRequest{
			DisableSafetyCheck:    *disableSafetyCheck,
			FailOnReboot:          *failOnReboot,
			ForceDisruptiveUpdate: *forceDisruptiveUpdate,
			Hostname:              subHostname,
			QueueTimeout:          *queueTimeout,
			Timeout:               *timeout,
			UsePlannedImage:       *usePlannedImage,
		},
		logger)
	if err != nil {
		return false, err
	}
	if *showFastUpdateResult {
		defer json.WriteWithIndent(os.Stdout, "    ", reply)
	}
	if reply.Synced {
		logger.Debugf(0,
			"finished in %s, queue time: %s, processing time: %s\n",
			format.Duration(time.Since(startTime)),
			format.Duration(reply.QueueTime),
			format.Duration(reply.ProcessingTime))
		return false, nil
	}
	logger.Debugf(0, "failed after %s\n",
		format.Duration(time.Since(startTime)))
	return reply.RebootBlocked, errors.New("unable to complete update")
}
