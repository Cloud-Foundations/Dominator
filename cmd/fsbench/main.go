package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/fsbench"
	"github.com/Cloud-Foundations/Dominator/lib/version"
)

// Benchmark the read speed of the underlying block device for a given file.
func main() {
	checkVersion := version.AddFlags("fsbench")
	flag.Parse()
	checkVersion()
	pathname := "/"
	if flag.NArg() == 1 {
		pathname = flag.Arg(0)
	}
	bytesPerSecond, blocksPerSecond, err := fsbench.GetReadSpeed(pathname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error! %s\n", err)
		return
	}
	fmt.Printf("speed=%d MiB/s ", bytesPerSecond>>20)
	if blocksPerSecond > 0 {
		fmt.Printf("%d blocks/s\n", blocksPerSecond)
	} else {
		fmt.Println("I/O accounting not available")
	}
}
