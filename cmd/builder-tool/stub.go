// +build !linux

package main

import (
	"errors"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

var errorNotAvailable = errors.New("not available on this OS")

func buildFromManifestSubcommand(args []string, logger log.DebugLogger) error {
	return errorNotAvailable
}

func buildRawFromManifestSubcommand(args []string,
	logger log.DebugLogger) error {
	return errorNotAvailable
}

func buildTreeFromManifestSubcommand(args []string,
	logger log.DebugLogger) error {
	return errorNotAvailable
}

func processManifestSubcommand(args []string, logger log.DebugLogger) error {
	return errorNotAvailable
}
