package main

import (
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	certificateRequestPath = "/v1/getRoleRequestingCert"
)

func replaceVmIdentitySubcommand(args []string,
	logger log.DebugLogger) error {
	if err := replaceVmIdentity(args[0], logger); err != nil {
		return fmt.Errorf("error replacing VM identity: %s", err)
	}
	return nil
}

func replaceVmIdentity(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return replaceVmIdentityOnHypervisor(hypervisor, vmIP, logger)
	}
}

func replaceVmIdentityOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return replaceVmIdentityOnConnectedHypervisor(client, hypervisor, ipAddr,
		logger)
}

func replaceVmIdentityOnConnectedHypervisor(client srpc.ClientI,
	hypervisorAddress string, vmIP net.IP, logger log.DebugLogger) error {
	if *identityName == "" {
		return hyperclient.ReplaceVmIdentity(client,
			hyper_proto.ReplaceVmIdentityRequest{IpAddress: vmIP})
	}
	hypervisorHostname, _, err := net.SplitHostPort(hypervisorAddress)
	if err != nil {
		return err
	}
	hypervisorIpList, err := net.LookupHost(hypervisorHostname)
	if err != nil {
		return err
	}
	hypervisorIpMap := stringutil.ConvertListToMap(hypervisorIpList, true)
	if *fleetManagerHostname != "" {
		fm := fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)
		fmClient, err := dialFleetManager(fm)
		if err != nil {
			return err
		}
		defer fmClient.Close()
		request := fm_proto.GetMachineInfoRequest{
			Hostname:               hypervisorHostname,
			IgnoreMissingLocalTags: true,
		}
		var reply fm_proto.GetMachineInfoResponse
		err = fmClient.RequestReply("FleetManager.GetMachineInfo", request,
			&reply)
		if err != nil {
			return err
		}
		if err := errors.New(reply.Error); err != nil {
			return err
		}
		if reply.Machine.HostIpAddress != nil {
			hypervisorIpMap[reply.Machine.HostIpAddress.String()] = struct{}{}
		}
		for _, netEntry := range reply.Machine.SecondaryNetworkEntries {
			if netEntry.HostIpAddress != nil {
				hypervisorIpMap[netEntry.HostIpAddress.String()] = struct{}{}
			}
		}
	}
	delete(hypervisorIpMap, "") // Just in case.
	identityProvider, err := hyperclient.GetIdentityProvider(client)
	if err != nil {
		return err
	}
	if identityProvider == "" {
		return fmt.Errorf("%s: has no Identity Provider", hypervisorAddress)
	}
	pubkeyPEM, err := hyperclient.GetPublicKey(client)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(pubkeyPEM)
	if block == nil {
		return errors.New("error decoding PEM public key")
	}
	if block.Type != "PUBLIC KEY" {
		return fmt.Errorf("unsupported public key type: \"%s\"", block.Type)
	}
	pubkeyDER := block.Bytes
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = srpc.GetClientTlsConfig()
	// TODO(rgooch): transport.TLSClientConfig.InsecureSkipVerify = false
	httpClient := http.Client{Transport: transport}
	baseUrl, err := url.Parse(identityProvider)
	if err != nil {
		return err
	}
	requestUrl := url.URL{
		Scheme: baseUrl.Scheme,
		Host:   baseUrl.Host,
		Path:   certificateRequestPath,
	}
	var requestorNetblocks []string
	for ip := range hypervisorIpMap {
		requestorNetblocks = append(requestorNetblocks, ip+"/32")
	}
	resp, err := httpClient.PostForm(
		requestUrl.String(),
		url.Values{
			"identity": []string{*identityName},
			"pubkey": []string{
				base64.RawURLEncoding.EncodeToString(pubkeyDER)},
			"requestor_netblock": requestorNetblocks,
			"target_netblock":    []string{vmIP.String() + "/32"},
		})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting role requesting certificate: %s",
			resp.Status)
	}
	certPEM, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading role requesting certificate: %s", err)
	}
	cert, _, err := x509util.ParseCertificatePEM(certPEM, logger)
	if err != nil {
		return err
	}
	username, err := x509util.GetUsername(cert)
	if err != nil {
		return err
	}
	logger.Debugf(0,
		"Received role requesting certificate for: %s, expires at: %s (in %s)\n",
		username,
		cert.NotAfter.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(cert.NotAfter)))
	return hyperclient.ReplaceVmIdentity(client,
		hyper_proto.ReplaceVmIdentityRequest{
			IdentityRequestorCertificate: certPEM,
			IpAddress:                    vmIP,
		})
}
