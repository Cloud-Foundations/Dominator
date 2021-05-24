package mounts

type MountEntry struct {
	Device     string
	MountPoint string
	Type       string
	Options    string
}

type MountTable struct {
	Entries []*MountEntry
}

func GetMountTable() (*MountTable, error) {
	return getMountTable()
}

func (mt *MountTable) FindEntry(path string) *MountEntry {
	return mt.findEntry(path)
}
