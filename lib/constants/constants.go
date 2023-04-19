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

	// SmallStack endpoints.
	SmallStackDataSource = "/datasource/SmallStack"
	MetadataEpochTime    = "/latest/dynamic/epoch-time"
	MetadataIdentityCert = "/latest/dynamic/instance-identity/X.509-certificate"
	MetadataIdentityDoc  = "/latest/dynamic/instance-identity/document"
	MetadataIdentityKey  = "/latest/dynamic/instance-identity/X.509-key"

	MetadataExternallyPatchable = "/latest/is-externally-patchable"

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
