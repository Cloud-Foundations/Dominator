package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func processMdbTemplateSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := processMdbTemplate(client); err != nil {
		return fmt.Errorf("error processing MDB template: %s", err)
	}
	return nil
}

func processMdbTemplate(client srpc.ClientI) error {
	if *templateFilename == "" {
		return fmt.Errorf("no template file specified")
	}
	tmpl, err := template.ParseFiles(*templateFilename)
	if err != nil {
		return err
	}
	request := mdbserver.GetMdbRequest{}
	var reply mdbserver.GetMdbResponse
	err = client.RequestReply("MdbServer.GetMdb", request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	for _, machine := range reply.Machines {
		if err := tmpl.Execute(os.Stdout, machine); err != nil {
			return err
		}
	}
	return nil
}
