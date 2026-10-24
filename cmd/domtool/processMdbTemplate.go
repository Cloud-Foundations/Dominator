package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
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
	reply, err := mdbclient.GetMdb(client)
	if err != nil {
		return err
	}
	for _, machine := range reply.Machines {
		if err := tmpl.Execute(os.Stdout, machine); err != nil {
			return err
		}
	}
	return nil
}
