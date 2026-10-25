package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	protoDir = flag.String("proto", "",
		"Directory of the Go package containing the request and reply types")
	rpcdDir = flag.String("rpcd", "",
		"Directory of the Go package containing the annotated SRPC methods")
)

// printScan writes a description of the methods and the types they use.
func printScan(methods []methodInfo, infos []typeInfo) {
	for _, method := range methods {
		fmt.Printf("method %s: %s -> %s", method.Name, method.RequestType,
			method.ResponseType)
		if method.NoRest {
			fmt.Print(" (no-rest)")
		}
		fmt.Println()
	}
	for _, info := range infos {
		fmt.Printf("message %s\n", info.Name)
		for _, field := range info.Fields {
			fmt.Printf("  %d %s %s\n", field.Number, field.Name,
				field.GoType)
		}
	}
}

// doMain scans the annotated methods and their types and prints them. It
// returns the process exit status.
func doMain() int {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: proto-gen -rpcd dir -proto dir\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *protoDir == "" || *rpcdDir == "" {
		flag.Usage()
		return 2
	}
	methods, err := findMethods(*rpcdDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var rootNames []string
	for _, method := range methods {
		rootNames = append(rootNames, method.RequestType, method.ResponseType)
	}
	infos, err := scanTypes(*protoDir, rootNames)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printScan(methods, infos)
	return 0
}

// main runs doMain and exits with its status.
func main() {
	os.Exit(doMain())
}
