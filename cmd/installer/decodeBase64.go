package main

import (
	"fmt"
	"encoding/base64"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func decodeBase64Subcommand(args []string, logger log.DebugLogger) error {
	if err := decodeBase64(logger); err != nil {
		return fmt.Errorf("error decoding base64 encoded data: %s", err)
	}
	return nil
}

func decodeBase64(logger log.DebugLogger) error {
	decoder := base64.NewDecoder(base64.StdEncoding, os.Stdin)
	_, err := io.Copy(os.Stdout, decoder)
	return err
}
