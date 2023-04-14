package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

const (
	linklocalAddress = "169.254.169.254"
	metadataUrl      = "http://" + linklocalAddress + "/"
	identityCert     = "latest/dynamic/instance-identity/X.509-certificate"
)

func showCertFile(filename string) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read certfile: %s\n", err)
		return
	}
	fmt.Println("Certificate:", filename+":")
	showCert(data)
}

func showCertMetadata() {
	client := http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(metadataUrl + identityCert)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	fmt.Println("Certificate:", "MetadataIdentity:")
	showCert(data)
}

func showCert(data []byte) {
	block, rest := pem.Decode(data)
	if block == nil {
		fmt.Fprintf(os.Stderr, "Failed to parse certificate PEM")
		return
	}
	if len(rest) > 0 {
		fmt.Fprintf(os.Stderr, "%d extra bytes in certfile\n", len(rest))
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse certificate: %s\n", err)
		return
	}
	now := time.Now()
	if notYet := cert.NotBefore.Sub(now); notYet > 0 {
		fmt.Fprintf(os.Stderr, "  Will not be valid for %s\n",
			format.Duration(notYet))
	}
	if expired := now.Sub(cert.NotAfter); expired > 0 {
		fmt.Fprintf(os.Stderr, "  Expired %s ago\n", format.Duration(expired))
	} else {
		fmt.Fprintf(os.Stderr, "  Valid until %s (%s from now)\n",
			cert.NotAfter, format.Duration(-expired))
	}
	username, err := x509util.GetUsername(cert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get username: %s\n", err)
		return
	}
	fmt.Printf("  Issued to: %s\n", username)
	permittedMethods, err := x509util.GetPermittedMethods(cert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get methods: %s\n", err)
		return
	}
	if len(permittedMethods) > 0 {
		fmt.Println("  Permitted methods:")
		showList(permittedMethods)
	} else {
		fmt.Println("  No methods are permitted")
	}
	groupList, err := x509util.GetGroupList(cert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get group list: %s\n", err)
		return
	}
	if len(groupList) > 0 {
		fmt.Println("  Group list:")
		showList(groupList)
	} else {
		fmt.Println("  No group memberships")
	}
}

func showList(list map[string]struct{}) {
	sortedList := stringutil.ConvertMapKeysToList(list, true)
	for _, entry := range sortedList {
		fmt.Println("   ", entry)
	}
}

func main() {
	if len(os.Args) == 1 {
		showCertMetadata()
		return
	}
	for _, filename := range os.Args[1:] {
		showCertFile(filename)
	}
}
