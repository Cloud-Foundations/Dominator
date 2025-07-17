package installer

const (
	FileSystemTypeExt4 = 0
	FileSystemTypeVfat = 1
)

type FileSystemType uint

type Partition struct {
	FileSystemLabel  string         `json:",omitempty"`
	FileSystemType   FileSystemType `json:",omitempty"`
	MountPoint       string         `json:",omitempty"`
	MinimumBytes     uint64         `json:",omitempty"`
	MinimumFreeBytes uint64         `json:",omitempty"`
}

type StorageLayout struct {
	BootDriveLayout          []Partition `json:",omitempty"`
	ExtraMountPointsBasename string      `json:",omitempty"`
	Encrypt                  bool        `json:",omitempty"`
	UseKexec                 bool        `json:",omitempty"`
}
