package mbr

import (
	"fmt"
	"os"
	"os/exec"
)

func decode(file *os.File) (*Mbr, error) {
	var mbr Mbr
	if _, err := file.ReadAt(mbr.raw[:], 0); err != nil {
		return nil, err
	}
	if mbr.raw[0x1FE] == 0x55 && mbr.raw[0x1FF] == 0xAA {
		return &mbr, nil
	}
	return nil, nil
}

func read32LE(address []byte) uint64 {
	return uint64(address[0]) +
		uint64(address[1])<<8 +
		uint64(address[2])<<16 +
		uint64(address[3])<<24
}

func write32LE(address []byte, value uint64) {
	address[0] = byte(value & 0xff)
	address[1] = byte((value >> 8) & 0xff)
	address[2] = byte((value >> 16) & 0xff)
	address[3] = byte((value >> 24) & 0xff)
}

func writeDefault(filename string, tableType TableType) error {
	label, err := tableType.lookupString()
	if err != nil {
		return err
	}
	fmt.Printf("making table type: %d (%s)\n", tableType, label)
	cmd := exec.Command("parted", "-s", "-a", "optimal", filename,
		"mklabel", label,
		"mkpart", "primary", "ext2", "1", "100%",
		"set", "1", "boot", "on",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error partitioning: %s: %s: %s",
			filename, err, output)
	}
	return nil
}

func (mbr *Mbr) getPartitionOffset(index uint) uint64 {
	partitionOffset := 0x1BE + 0x10*index
	return 512 * read32LE(mbr.raw[partitionOffset+8:])
}

func (mbr *Mbr) getPartitionSize(index uint) uint64 {
	partitionOffset := 0x1BE + 0x10*index
	return 512 * read32LE(mbr.raw[partitionOffset+12:])
}

func (mbr *Mbr) setPartitionOffset(index uint, offset uint64) error {
	if index >= mbr.GetNumPartitions() {
		return fmt.Errorf("invalid partition index: %d", index)
	}
	offsetSector := offset >> 9
	if offsetSector<<9 != offset {
		return fmt.Errorf("offset: %d is not an integral multiple of blocks",
			offset)
	}
	partitionOffset := 0x1BE + 0x10*index
	write32LE(mbr.raw[partitionOffset+8:], offsetSector)
	return nil
}

func (mbr *Mbr) setPartitionSize(index uint, size uint64) error {
	if index >= mbr.GetNumPartitions() {
		return fmt.Errorf("invalid partition index: %d", index)
	}
	sizeSector := size >> 9
	if sizeSector<<9 != size {
		return fmt.Errorf("size: %d is not an integral multiple of blocks",
			size)
	}
	partitionOffset := 0x1BE + 0x10*index
	write32LE(mbr.raw[partitionOffset+12:], sizeSector)
	return nil
}

func (mbr *Mbr) write(filename string) error {
	if file, err := os.OpenFile(filename, os.O_WRONLY, 0622); err != nil {
		return err
	} else {
		defer file.Close()
		if length, err := file.Write(mbr.raw[:]); err != nil {
			return err
		} else if length != len(mbr.raw) {
			return fmt.Errorf("short write: %d", length)
		}
		return nil
	}
}
