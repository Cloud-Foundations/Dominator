package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

var (
	goPackage = flag.String("goPackage", "",
		"Import path of the generated Go code (with -out)")
	outFile = flag.String("out", "",
		"File to write the generated .proto to. If empty, print the scan")
	protoDir = flag.String("proto", "",
		"Directory of the Go package containing the request and reply types")
	protoPackage = flag.String("package", "",
		"Name of the proto package (with -out)")
	rpcdDir = flag.String("rpcd", "",
		"Directory of the Go package containing the annotated SRPC methods")
	serviceName = flag.String("service", "",
		"Name of the gRPC service (with -out)")
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

// checkOutFlags returns an error if the generation flags are inconsistent:
// -goPackage, -package and -service must all be set with -out and none
// without it.
func checkOutFlags(out, goPkg, protoPkg, service string) error {
	generating := goPkg != "" || protoPkg != "" || service != ""
	if out == "" && generating {
		return errors.New("-goPackage, -package and -service need -out")
	}
	if out != "" && (goPkg == "" || protoPkg == "" || service == "") {
		return errors.New(
			"-goPackage, -package and -service are required with -out")
	}
	return nil
}

// doMain scans the annotated methods and their types. It writes the .proto
// file if -out is given and prints the scan otherwise. It returns the process
// exit status.
func doMain() int {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: proto-gen -rpcd dir -proto dir")
		fmt.Fprintln(os.Stderr,
			"       [-out file -package name -goPackage path -service name]")
		flag.PrintDefaults()
	}
	err := loadflags.LoadForCli("proto-gen")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Parse()
	if *protoDir == "" || *rpcdDir == "" {
		flag.Usage()
		return 2
	}
	err = checkOutFlags(*outFile, *goPackage, *protoPackage, *serviceName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		return 2
	}
	methods, err := findMethods(*rpcdDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	infos, err := scanTypes(*protoDir, rootTypeNames(methods))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *outFile == "" {
		printScan(methods, infos)
		return 0
	}
	named, err := scanNamedTypes(*protoDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	config := protoConfig{
		GoPackage: *goPackage,
		Package:   *protoPackage,
		Service:   *serviceName,
	}
	text, err := generateProto(config, methods, infos, named)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = os.WriteFile(*outFile, []byte(text), fsutil.PublicFilePerms)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// main runs doMain and exits with its status.
func main() {
	os.Exit(doMain())
}
