package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	dm_proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
	sub_proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

func disruptionCancelSubcommand(args []string, logger log.DebugLogger) error {
	err := disruptionOperation(sub_proto.DisruptionRequestCancel, args[0])
	if err != nil {
		return fmt.Errorf("error cancelling disruption: %s", err)
	}
	return nil
}

func disruptionCheckSubcommand(args []string, logger log.DebugLogger) error {
	err := disruptionOperation(sub_proto.DisruptionRequestCheck, args[0])
	if err != nil {
		return fmt.Errorf("error checking disruption state: %s", err)
	}
	return nil
}

func disruptionRequestSubcommand(args []string, logger log.DebugLogger) error {
	err := disruptionOperation(sub_proto.DisruptionRequestRequest, args[0])
	if err != nil {
		return fmt.Errorf("error requesting disruption: %s", err)
	}
	return nil
}

func disruptionOperation(requestType sub_proto.DisruptionRequest,
	subHostname string) error {
	machine, err := getMachineMdb(subHostname)
	if err != nil {
		return err
	}
	parsedUrl, err := url.Parse(*disruptionManagerUrl)
	if err != nil {
		return err
	}
	switch parsedUrl.Scheme {
	case "http", "https":
		data := &bytes.Buffer{}
		err := json.WriteWithIndent(data, "    ",
			dm_proto.DisruptionRequest{MDB: machine, Request: requestType})
		if err != nil {
			return err
		}
		resp, err := http.Post(*disruptionManagerUrl, "application/json", data)
		if err != nil {
			return fmt.Errorf("POST error: %s", err)
		}
		defer resp.Body.Close()
		var reply dm_proto.DisruptionResponse
		if resp.StatusCode != http.StatusOK {
			body := &strings.Builder{}
			io.Copy(body, resp.Body)
			return fmt.Errorf("%s: %s",
				resp.Status, strings.TrimSpace(body.String()))
		}
		if err := json.Read(resp.Body, &reply); err != nil {
			return fmt.Errorf("error decoding response: %s", err)
		}
		fmt.Println(reply.Response)
		return nil
	case "srpc":
		client, err := srpc.DialHTTP("tcp", parsedUrl.Host, 0)
		if err != nil {
			return fmt.Errorf("error dialing: %s", err)
		}
		defer client.Close()
		switch requestType {
		case sub_proto.DisruptionRequestCancel:
			request := dm_proto.DisruptionCancelRequest{MDB: machine}
			var reply dm_proto.DisruptionCancelResponse
			err := client.RequestReply("DisruptionManager.Cancel",
				request, &reply)
			if err != nil {
				return err
			}
			if err := errors.New(reply.Error); err != nil {
				return err
			}
			fmt.Println(reply.Response)
		case sub_proto.DisruptionRequestCheck:
			request := dm_proto.DisruptionCheckRequest{MDB: machine}
			var reply dm_proto.DisruptionCheckResponse
			err := client.RequestReply("DisruptionManager.Check",
				request, &reply)
			if err != nil {
				return err
			}
			if err := errors.New(reply.Error); err != nil {
				return err
			}
			fmt.Println(reply.Response)
		case sub_proto.DisruptionRequestRequest:
			request := dm_proto.DisruptionRequestRequest{MDB: machine}
			var reply dm_proto.DisruptionRequestResponse
			err := client.RequestReply("DisruptionManager.Request",
				request, &reply)
			if err != nil {
				return err
			}
			if err := errors.New(reply.Error); err != nil {
				return err
			}
			fmt.Println(reply.Response)
		}
	default:
		return fmt.Errorf("unsupported scheme: %s", *disruptionManagerUrl)
	}
	return nil
}
