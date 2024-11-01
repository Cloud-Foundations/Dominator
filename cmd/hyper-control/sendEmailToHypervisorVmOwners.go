package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/net/smtp"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupclient"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/text"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func sendEmailToHypervisorVmOwnersSubcommand(args []string,
	logger log.DebugLogger) error {
	err := sendEmailToHypervisorVmOwners(logger)
	if err != nil {
		return fmt.Errorf("error sending email: %s", err)
	}
	return nil
}

func getHypervisorVmInfos() ([]hyper_proto.VmInfo, error) {
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	stateMask := uint64(1) << hyper_proto.StateRunning
	vmInfos, err := hyperclient.GetVmInfos(client,
		hyper_proto.GetVmInfosRequest{
			IgnoreStateMask: ^stateMask,
		})
	if err != nil {
		return nil, err
	}
	return vmInfos, nil
}

func sendEmailToHypervisorVmOwners(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return fmt.Errorf("no hypervisorHostname specified")
	}
	var body []byte
	var err error
	if *emailBodyFilename == "" {
		fmt.Fprintln(os.Stderr, "Enter body text:")
		body, err = io.ReadAll(os.Stdin)
	} else {
		body, err = os.ReadFile(*emailBodyFilename)
	}
	if err != nil {
		return err
	}
	if len(body) < 1 {
		return fmt.Errorf("no message body")
	}
	tlsCerts, err := srpc.LoadCertificates(setupclient.GetCertDirectory())
	if err != nil {
		return err
	}
	username, err := x509util.GetUsername(tlsCerts[0].Leaf)
	if err != nil {
		return err
	}
	from := username + "@" + *emailDomain
	vmInfos, err := getHypervisorVmInfos()
	if err != nil {
		return err
	}
	// Get list of all owner users.
	toUsers := make(map[string]struct{})
	for _, vmInfo := range vmInfos {
		for _, ownerUser := range vmInfo.OwnerUsers {
			toUsers[ownerUser] = struct{}{}
		}
	}
	message := &bytes.Buffer{}
	fmt.Fprintf(message, "From: %s\n", from)
	fmt.Fprintf(message, "To: %s\n", from)
	fmt.Fprintf(message, "Subject: "+*emailSubject+"\n", *hypervisorHostname)
	fmt.Fprintln(message) // Separator between headers and body.
	message.Write(body)
	var plural string
	if len(vmInfos) > 1 {
		plural = "s"
	}
	fmt.Fprintf(message, "The following %d running VM%s will be affected:\n\n",
		len(vmInfos), plural)
	fmt.Fprintln(message,
		"IP               HOSTNAME                             OWNER")
	// First sort by IP address.
	sort.Slice(vmInfos, func(i, j int) bool {
		return verstr.Less(vmInfos[i].Address.IpAddress.String(),
			vmInfos[j].Address.IpAddress.String())
	})
	// Now sort by owner. End result is an IP-sorted sublist for each owner.
	sort.SliceStable(vmInfos, func(i, j int) bool {
		return vmInfos[i].OwnerUsers[0] < vmInfos[j].OwnerUsers[0]
	})
	columnCollector := &text.ColumnCollector{}
	for _, vmInfo := range vmInfos {
		columnCollector.AddField(vmInfo.Address.IpAddress.String())
		columnCollector.AddField(vmInfo.Hostname)
		columnCollector.AddField(vmInfo.OwnerUsers[0])
		columnCollector.CompleteLine()
	}
	if err := columnCollector.WriteLeftAligned(message); err != nil {
		return err
	}
	return smtp.SendMailPlain(*smtpServer, *emailDomain,
		from,
		stringutil.ConvertMapKeysToList(toUsers, true),
		message.Bytes())
}
