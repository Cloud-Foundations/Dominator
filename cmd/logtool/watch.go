package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/logger"
)

func watchSubcommand(args []string, logger log.DebugLogger) error {
	level, err := strconv.ParseInt(args[0], 10, 16)
	if err != nil {
		return fmt.Errorf("error parsing level: %s", err)
	}
	clients, addrs, err := dial(true)
	if err != nil {
		return err
	}
	if err := watchAll(clients, addrs, int16(level)); err != nil {
		return fmt.Errorf("error watching: %s", err)
	}
	return nil
}

func watchAll(clients []*srpc.Client, addrs []string, level int16) error {
	if len(clients) == 1 {
		return watchOne(clients[0], level, "")
	}
	maxWidth := 0
	for _, addr := range addrs {
		if len(addr) > maxWidth {
			maxWidth = len(addr)
		}
	}
	errors := make(chan error, 1)
	for index, client := range clients {
		prefix := addrs[index]
		if len(prefix) < maxWidth {
			prefix += strings.Repeat(" ", maxWidth-len(prefix))
		}
		go func(client *srpc.Client, level int16, prefix string) {
			errors <- watchOne(client, level, prefix)
		}(client, level, prefix)
	}
	for range clients {
		if err := <-errors; err != nil {
			return err
		}
	}
	return nil
}

func watchOne(client *srpc.Client, level int16, prefix string) error {
	request := proto.WatchRequest{
		ExcludeRegex: *excludeRegex,
		IncludeRegex: *includeRegex,
		Name:         *loggerName,
		DebugLevel:   level,
	}
	if conn, err := client.Call("Logger.Watch"); err != nil {
		return err
	} else {
		defer conn.Close()
		if err := conn.Encode(request); err != nil {
			return err
		}
		if err := conn.Flush(); err != nil {
			return err
		}
		var response proto.WatchResponse
		if err := conn.Decode(&response); err != nil {
			return fmt.Errorf("error decoding: %s", err)
		}
		if response.Error != "" {
			return errors.New(response.Error)
		}
		if prefix == "" {
			_, err := io.Copy(os.Stdout, conn)
			return err
		}
		for {
			line, err := conn.ReadString('\n')
			if len(line) > 0 {
				if prefix != "" {
					line = prefix + " " + line
				}
				if _, err := os.Stdout.Write([]byte(line)); err != nil {
					return err
				}
			}
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	}
}
