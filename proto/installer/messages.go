package installer

import (
	"github.com/Cloud-Foundations/Dominator/lib/types"
)

const (
	FileSystemTypeExt4 = 0
	FileSystemTypeVfat = 1
)

type FileSystemType uint

type Partition struct {
	BytesPerInode            types.Bytes      `json:",omitempty"`
	FileSystemLabel          string           `json:",omitempty"`
	FileSystemType           FileSystemType   `json:",omitempty"`
	MountPoint               string           `json:",omitempty"`
	MinimumBytes             uint64           `json:",omitempty"`
	MinimumFreeBytes         uint64           `json:",omitempty"`
	ReservedBlocksPercentage types.Percentage `json:",omitempty"`
	RootGroupId              types.GroupId    `json:",omitempty"`
	RootUserId               types.UserId     `json:",omitempty"`
}

type StorageLayout struct {
	BootDriveLayout          []Partition `json:",omitempty"`
	ExtraMountPointsBasename string      `json:",omitempty"`
	Encrypt                  bool        `json:",omitempty"`
	UseKexec                 bool        `json:",omitempty"`
}
