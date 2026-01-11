package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/cmdlogger"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

type certType struct {
	expiresAt time.Time
	filename  string
}

func loadCertFile(filename string, logger log.DebugLogger) (*certType, error) {
	cert, _, err := x509util.LoadCertificatePEM(filename, logger)
	if err != nil {
		return nil, err
	}
	return &certType{
		expiresAt: cert.NotAfter,
		filename:  filename,
	}, nil
}

func doMain() int {
	err := loadflags.LoadForCli("list-cert-expirations")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Parse()
	logger := cmdlogger.New()
	var certificates []*certType
	for _, filename := range flag.Args() {
		if cert, err := loadCertFile(filename, logger); err != nil {
			return 1
		} else {
			certificates = append(certificates, cert)
		}
	}
	sort.Slice(certificates, func(i, j int) bool {
		return certificates[i].expiresAt.Before(certificates[j].expiresAt)
	})
	var maxWidth int
	for _, cert := range certificates {
		if len(cert.filename) > maxWidth {
			maxWidth = len(cert.filename)
		}
	}
	now := time.Now()
	for _, cert := range certificates {
		if expired := now.Sub(cert.expiresAt); expired > 0 {
			fmt.Printf("%*s EXPIRED %s ago\n",
				maxWidth, cert.filename, format.Duration(expired))
		} else {
			fmt.Printf("%*s expires %s (%s from now)\n",
				maxWidth, cert.filename,
				cert.expiresAt, format.Duration(-expired))
		}
	}
	return 0
}

func main() {
	os.Exit(doMain())
}
