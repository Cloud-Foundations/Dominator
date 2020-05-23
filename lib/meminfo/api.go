package meminfo

type MemInfo struct {
	Available     uint64
	Free          uint64
	HaveAvailable bool
	Total         uint64
}

func GetMemInfo() (*MemInfo, error) {
	return getMemInfo()
}
