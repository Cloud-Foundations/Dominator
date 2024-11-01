package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/text"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

type expirationData struct {
	duration string
	time     string
}

func showBadImageSubsSubcommand(args []string,
	logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	mdbdSClient, err := dialMdbd()
	if err != nil {
		return err
	}
	if err := showBadImageSubs(imageSClient, mdbdSClient); err != nil {
		return fmt.Errorf("error showing subs with expiring/missing images: %s",
			err)
	}
	return nil
}

func showBadImageSubs(imageSClient, mdbdSClient srpc.ClientI) error {
	// Get list of good images.
	imageNames, err := client.ListSelectedImages(imageSClient,
		imageserver.ListSelectedImagesRequest{IgnoreExpiringImages: true})
	if err != nil {
		return err
	}
	goodImages := stringutil.ConvertListToMap(imageNames, false)
	// Get data from MDB.
	request := mdbserver.GetMdbRequest{}
	var reply mdbserver.GetMdbResponse
	err = mdbdSClient.RequestReply("MdbServer.GetMdb", request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	// Loop over machines, skipping those with good images.
	imageExpirations := make(map[string]*expirationData)
	columnCollector := &text.ColumnCollector{}
	for _, machine := range reply.Machines {
		if machine.RequiredImage == "" {
			continue
		}
		if _, ok := goodImages[machine.RequiredImage]; ok {
			continue
		}
		imageExpiration := imageExpirations[machine.RequiredImage]
		if imageExpiration == nil {
			expiresAt, err := client.GetImageExpiration(imageSClient,
				machine.RequiredImage)
			if err != nil {
				if !strings.Contains(err.Error(), "image not found") {
					return err
				}
				imageExpiration = &expirationData{
					time: "MISSING",
				}
			} else {
				imageExpiration = &expirationData{
					duration: format.Duration(time.Until(expiresAt)),
					time:     expiresAt.Format(format.TimeFormatSeconds),
				}
			}
			imageExpirations[machine.RequiredImage] = imageExpiration
		}
		columnCollector.AddField(machine.Hostname)
		columnCollector.AddField(machine.RequiredImage)
		columnCollector.AddField(imageExpiration.time)
		columnCollector.AddField(imageExpiration.duration)
		columnCollector.CompleteLine()
	}
	return columnCollector.WriteLeftAligned(os.Stdout)
}
