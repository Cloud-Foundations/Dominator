package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func generateRandomSubcommand(args []string, logger log.DebugLogger) error {
	if err := generateRandom(logger); err != nil {
		return fmt.Errorf("error generating random data: %s", err)
	}
	return nil
}

func generateRandom(logger log.DebugLogger) error {
	generator := rand.New(
		rand.NewSource(time.Now().Unix() + time.Now().UnixNano()))
	buffer := make([]byte, 1<<20) // 1 MiB.
	for {
		generator.Read(buffer)
		if _, err := os.Stdout.Write(buffer); err != nil {
			return err
		}
	}
}
