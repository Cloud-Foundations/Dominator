package sub

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

type Configuration struct {
	CpuPercent          uint
	OwnerGroups         []string
	OwnerUsers          []string
	NetworkSpeedPercent uint
	ScanSpeedPercent    uint
	ScanExclusionList   []string
}

type FileToCopyToCache struct {
	Name       string
	Hash       hash.Hash
	DoHardlink bool
}

type Hardlink struct {
	NewLink string
	Target  string
}

type Inode struct {
	Name string
	filesystem.GenericInode
}
