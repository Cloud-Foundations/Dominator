package constants

const (
	SubPortNumber                = 6969
	DominatorPortNumber          = 6970
	ImageServerPortNumber        = 6971
	BasicFileGenServerPortNumber = 6972
	SimpleMdbServerPortNumber    = 6973
	ImageUnpackerPortNumber      = 6974
	ImaginatorPortNumber         = 6975
	HypervisorPortNumber         = 6976
	FleetManagerPortNumber       = 6977
	InstallerPortNumber          = 6978
	DisruptionManagerPortNumber  = 6979

	DefaultCpuPercent          = 50
	DefaultNetworkSpeedPercent = 10
	DefaultScanSpeedPercent    = 2

	AssignedOIDBase        = "1.3.6.1.4.1.9586.100.7"
	PermittedMethodListOID = AssignedOIDBase + ".1"
	GroupListOID           = AssignedOIDBase + ".2"

	DefaultMdbFile = "/var/lib/mdbd/mdb.json"

	InitialImageNameFile = "/var/lib/initial-image"
	PatchedImageNameFile = "/var/lib/patched-image"

	// Metadata service.
	LinklocalAddress = "169.254.169.254"
	MetadataUrl      = "http://" + LinklocalAddress

	// Common endpoints.
	MetadataUserData = "/latest/user-data"

	// SmallStack Metadata Service endpoints.
	SmallStackDataSource        = "/datasource/SmallStack"
	MetadataEpochTime           = "/latest/dynamic/epoch-time"
	MetadataIdentityDoc         = "/latest/dynamic/instance-identity/document"
	MetadataExternallyPatchable = "/latest/is-externally-patchable"
	// SmallStack identity credential endpoints.
	// Default: RSA X.509.
	MetadataIdentityCert = "/latest/dynamic/instance-identity/X.509-certificate"
	MetadataIdentityKey  = "/latest/dynamic/instance-identity/X.509-key"
	// Ed25519 SSH & RSA.
	MetadataIdentityEd25519SshCert  = "/latest/dynamic/instance-identity/Ed25519-SSH-certificate"
	MetadataIdentityEd25519SshKey   = "/latest/dynamic/instance-identity/Ed25519-SSH-key"
	MetadataIdentityEd25519X509Cert = "/latest/dynamic/instance-identity/Ed25519-X.509-certificate"
	MetadataIdentityEd25519X509Key  = "/latest/dynamic/instance-identity/Ed25519-X.509-key"
	// RSA SSH & X.509.
	MetadataIdentityRsaSshCert  = "/latest/dynamic/instance-identity/RSA-SSH-certificate"
	MetadataIdentityRsaSshKey   = "/latest/dynamic/instance-identity/RSA-SSH-key"
	MetadataIdentityRsaX509Cert = "/latest/dynamic/instance-identity/RSA-X.509-certificate"
	MetadataIdentityRsaX509Key  = "/latest/dynamic/instance-identity/RSA-X.509-key"

	// AWS endpoints.
	MetadataAwsInstanceType = "/latest/meta-data/instance-type"
)

var RequiredPaths = map[string]rune{
	"/etc":        'd',
	"/etc/passwd": 'f',
	"/usr":        'd',
	"/usr/bin":    'd',
}

var ScanExcludeList = []string{
	"/data/.*",
	"/home/.*",
	"/tmp/.*",
	"/var/log/.*",
	"/var/mail/.*",
	"/var/spool/.*",
	"/var/tmp/.*",
}
