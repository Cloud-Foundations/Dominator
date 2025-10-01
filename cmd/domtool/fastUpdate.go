package main

import (
	"fmt"
	"time"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	domproto "github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func fastUpdateSubcommand(args []string, logger log.DebugLogger) error {
	if err := fastUpdate(getClient(), args[0], logger); err != nil {
		return fmt.Errorf("error doing fast update: %s", err)
	}
	return nil
}

func fastUpdate(client *srpc.Client, subHostname string,
	logger log.DebugLogger) error {
	startTime := time.Now()
	logger = prefixlogger.New(subHostname+": ", logger)
	synced, err := domclient.FastUpdate(client, domproto.FastUpdateRequest{
		DisableSafetyCheck:    *disableSafetyCheck,
		FailOnReboot:          *failOnReboot,
		ForceDisruptiveUpdate: *forceDisruptiveUpdate,
		Hostname:              subHostname,
		Timeout:               *timeout,
		UsePlannedImage:       *usePlannedImage,
	},
		logger)
	if err != nil {
		return err
	}
	logger.Debugf(0, "finished in %s\n", format.Duration(time.Since(startTime)))
	if synced {
		return nil
	}
	return errors.New("unable to complete update")
}
