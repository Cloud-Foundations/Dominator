package main

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/cmdlogger"
	"github.com/Cloud-Foundations/Dominator/lib/sshutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/version"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
)

type ipAdressFamily struct {
	AddressFamily []byte
	Addresses     []asn1.BitString
}

type metadataCertificateType struct {
	certType string
	path     string
	isSsh    bool
}

var (
	// Flags.
	caFile = flag.String("CAfile", "/etc/ssl/CA.pem",
		"Name of file containing the root of trust for identity and methods")
	identityCaFile = flag.String("identityCAfile", "/etc/ssl/IdentityCA.pem",
		"Name of file containing the root of trust for identity only")

	ipV4FamilyEncoding = []byte{0, 1, 1} //NOTE this is only for unicast addresses
	ipV6FamilyEncoding = []byte{0, 2}

	metadataCertificates = []metadataCertificateType{
		{"RSA/X.509", constants.MetadataIdentityCert, false},
		{"RSA/SSH", constants.MetadataIdentityRsaSshCert, true},
		{"Ed25519/X.509", constants.MetadataIdentityEd25519X509Cert, false},
		{"Ed25519/SSH", constants.MetadataIdentityEd25519SshCert, true},
	}

	oidIPAddressDelegation = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 7}
	x509CaPoolMap          = make(map[string]*x509.CertPool)
)

func decodeIPV4Address(encodedBlock asn1.BitString) (net.IPNet, error) {
	var encodedIP [4]byte
	if encodedBlock.BitLength < 1 || encodedBlock.BitLength > 32 {
		failval := net.IPNet{}
		return failval, fmt.Errorf("invalid encoded bit length")
	}
	for i := 0; (i*8) < encodedBlock.BitLength && i < len(encodedBlock.Bytes); i++ {
		encodedIP[i] = encodedBlock.Bytes[i]
	}
	netBlock := net.IPNet{
		IP:   net.IPv4(encodedIP[0], encodedIP[1], encodedIP[2], encodedIP[3]),
		Mask: net.CIDRMask(encodedBlock.BitLength, 32),
	}
	return netBlock, nil
}

func decodeIPV6Address(encodedBlock asn1.BitString) (net.IPNet, error) {
	encodedIP := make([]byte, 16)
	if encodedBlock.BitLength < 1 || encodedBlock.BitLength > 128 {
		failval := net.IPNet{}
		return failval, fmt.Errorf("invalid encoded bit length")
	}
	for i := 0; (i*8) < encodedBlock.BitLength && i < len(encodedBlock.Bytes); i++ {
		encodedIP[i] = encodedBlock.Bytes[i]
	}
	netBlock := net.IPNet{
		IP:   encodedIP,
		Mask: net.CIDRMask(encodedBlock.BitLength, 128),
	}
	return netBlock, nil
}

func getFromMetadata(path string) ([]byte, error) {
	client := http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(constants.MetadataUrl + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func loadX509CAs(filename string, caPoolMap map[string]*x509.CertPool) error {
	if filename == "" {
		return nil
	}
	caList, _, err := x509util.LoadCertificatePEMs(filename)
	if len(caList) < 1 {
		return nil
	}
	if err == nil {
		certPool := x509.NewCertPool()
		for _, ca := range caList {
			certPool.AddCert(ca)
		}
		caPoolMap[filename] = certPool
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func logIpRestrictions(value []byte, logger log.DebugLogger) {
	var ipAddressFamilyList []ipAdressFamily
	if _, err := asn1.Unmarshal(value, &ipAddressFamilyList); err != nil {
		return
	}
	for _, addressList := range ipAddressFamilyList {
		if bytes.Equal(addressList.AddressFamily, ipV4FamilyEncoding) {
			for _, encodedNetblock := range addressList.Addresses {
				netBlock, err := decodeIPV4Address(encodedNetblock)
				if err != nil {
					logger.Println(err)
					continue
				}
				numOnes, _ := netBlock.Mask.Size()
				var mask string
				if numOnes > 0 {
					mask = strconv.Itoa(numOnes)
				} else {
					mask = netBlock.Mask.String()
				}
				fmt.Printf("  Restricted to IP: %s/%s\n", netBlock.IP, mask)
			}
			continue
		}
		if bytes.Equal(addressList.AddressFamily, ipV6FamilyEncoding) {
			for _, encodedNetblock := range addressList.Addresses {
				netBlock, err := decodeIPV6Address(encodedNetblock)
				if err != nil {
					logger.Println(err)
					continue
				}
				numOnes, _ := netBlock.Mask.Size()
				var mask string
				if numOnes > 0 {
					mask = strconv.Itoa(numOnes)
				} else {
					mask = netBlock.Mask.String()
				}
				fmt.Printf("  Restricted to IP: %s/%s\n", netBlock.IP, mask)
			}
			continue
		}
		logger.Printf(
			"IP restriction extension: invalid/unknown address family: %s\n",
			addressList.AddressFamily)
	}
}

func showCertFile(filename string, logger log.DebugLogger) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read certfile: %s\n", err)
		return
	}
	fmt.Println("Certificate:", filename+":")
	showX509Cert(data, logger)
}

func showCertMetadata(logger log.DebugLogger) {
	for _, mCert := range metadataCertificates {
		data, err := getFromMetadata(mCert.path)
		if err != nil {
			return
		}
		fmt.Println("Certificate:", "MetadataIdentity:", mCert.certType+":")
		if mCert.isSsh {
			showSshCert(data)
		} else {
			showX509Cert(data, logger)
		}
	}
}

func showList(list map[string]struct{}) {
	sortedList := stringutil.ConvertMapKeysToList(list, true)
	for _, entry := range sortedList {
		fmt.Println("   ", entry)
	}
}

func showSshCert(data []byte) {
	_, cert, _, err := sshutil.ParseCertificate(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	notAfter := time.Unix(int64(cert.ValidBefore), 0)
	notBefore := time.Unix(int64(cert.ValidAfter), 0)
	now := time.Now()
	if notYet := notBefore.Sub(now); notYet > 0 {
		fmt.Printf("  Will not be valid for %s\n", format.Duration(notYet))
	}
	if expired := now.Sub(notAfter); expired > 0 {
		fmt.Printf("  Expired %s ago\n", format.Duration(expired))
	} else {
		fmt.Printf("  Valid until %s (%s from now)\n",
			notAfter, format.Duration(-expired))
	}
	fmt.Printf("  Principals: %v\n", cert.ValidPrincipals)
}

func showX509Cert(data []byte, logger log.DebugLogger) {
	cert, headers, err := x509util.ParseCertificatePEM(data, logger)
	if err != nil {
		logger.Println(err)
		return
	}
	now := time.Now()
	var invalid bool
	if notYet := cert.NotBefore.Sub(now); notYet > 0 {
		fmt.Printf("  Will not be valid for %s\n", format.Duration(notYet))
		invalid = true
	}
	if expired := now.Sub(cert.NotAfter); expired > 0 {
		fmt.Printf("  Expired %s ago\n", format.Duration(expired))
		invalid = true
	} else if !invalid {
		fmt.Printf("  Valid until %s (%s from now)\n",
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
	for _, extension := range cert.Extensions {
		if !extension.Id.Equal(oidIPAddressDelegation) {
			continue
		}
		logIpRestrictions(extension.Value, logger)
	}
	if len(headers) > 0 {
		fmt.Printf("  PEM headers: %v\n", headers)
	}
	if invalid {
		return
	}
	for filename, caPool := range x509CaPoolMap {
		opts := x509.VerifyOptions{
			Roots:     caPool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		chains, _ := cert.Verify(opts)
		if len(chains) > 0 {
			fmt.Printf("  Verified by CA in: %s\n", filename)
		}
	}
}

func doMain() int {
	checkVersion := version.AddFlags("show-cert")
	err := loadflags.LoadForCli("show-cert")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Parse()
	checkVersion()
	logger := cmdlogger.New()
	if err := loadX509CAs(*caFile, x509CaPoolMap); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := loadX509CAs(*identityCaFile, x509CaPoolMap); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if flag.NArg() < 1 {
		showCertMetadata(logger)
		return 1
	}
	for _, filename := range flag.Args() {
		showCertFile(filename, logger)
	}
	return 0
}

func main() {
	os.Exit(doMain())
}
