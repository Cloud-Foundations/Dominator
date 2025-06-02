package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func loadConfigurationFromTftpSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := loadConfigurationFromTftp(logger); err != nil {
		return fmt.Errorf("error loading configuration from TFTP: %s", err)
	}
	return nil
}

func loadConfigurationFromTftp(logger log.DebugLogger) error {
	if *tftpServerHostname == "" {
		return fmt.Errorf("no -tftpServerHostname specified")
	}
	return loadTftpFiles(*tftpServerHostname, logger)
}
