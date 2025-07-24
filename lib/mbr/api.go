package mbr

import (
	"os"
)

const (
	_ = TableType(iota)
	TABLE_TYPE_AIX
	TABLE_TYPE_AMIGA
	TABLE_TYPE_BSD
	TABLE_TYPE_DVH
	TABLE_TYPE_GPT
	TABLE_TYPE_LOOP
	TABLE_TYPE_MAC
	TABLE_TYPE_MSDOS
	TABLE_TYPE_PC98
	TABLE_TYPE_SUN
)

type Mbr struct {
	raw [512]byte
}

type TableType uint

func Decode(file *os.File) (*Mbr, error) {
	return decode(file)
}

func (mbr *Mbr) GetNumPartitions() uint {
	return 4
}

func (mbr *Mbr) GetPartitionOffset(index uint) uint64 {
	return mbr.getPartitionOffset(index)
}

func (mbr *Mbr) GetPartitionSize(index uint) uint64 {
	return mbr.getPartitionSize(index)
}

func (tt *TableType) Set(value string) error {
	return tt.set(value)
}

func (mbr *Mbr) SetPartitionOffset(index uint, offset uint64) error {
	return mbr.setPartitionOffset(index, offset)
}

func (mbr *Mbr) SetPartitionSize(index uint, size uint64) error {
	return mbr.setPartitionSize(index, size)
}

func (mbr *Mbr) Write(filename string) error {
	return mbr.write(filename)
}

func (tt TableType) String() string {
	return tt.string()
}

// WriteDefault will write a default partition table for the specified partition
// table type.
// DOS: a single ext2 Linux partition is created.
// GPT: a UEFI partition followed by an ext2 partition are created.
// Other partition types have unspecified behaviour.
func WriteDefault(filename string, tableType TableType) error {
	return writeDefault(filename, tableType)
}
