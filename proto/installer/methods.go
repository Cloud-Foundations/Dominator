package installer

import (
	"errors"
)

const (
	fileSystemTypeUnknown = "UNKNOWN FileSystemType"
)

var (
	fileSystemTypeToText = map[FileSystemType]string{
		FileSystemTypeExt4: "ext4",
		FileSystemTypeVfat: "vfat",
	}
	textToFileSystemType map[string]FileSystemType
)

func init() {
	textToFileSystemType = make(map[string]FileSystemType,
		len(fileSystemTypeToText))
	for fileSystemType, text := range fileSystemTypeToText {
		textToFileSystemType[text] = fileSystemType
	}
}

func (fileSystemType FileSystemType) MarshalText() ([]byte, error) {
	if text := fileSystemType.String(); text == fileSystemTypeUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (fileSystemType *FileSystemType) Set(value string) error {
	if val, ok := textToFileSystemType[value]; !ok {
		return errors.New(fileSystemTypeUnknown)
	} else {
		*fileSystemType = val
		return nil
	}
}

func (fileSystemType FileSystemType) String() string {
	if str, ok := fileSystemTypeToText[fileSystemType]; !ok {
		return fileSystemTypeUnknown
	} else {
		return str
	}
}

func (fileSystemType *FileSystemType) UnmarshalText(text []byte) error {
	return fileSystemType.Set(string(text))
}

func (left *Partition) Equal(right *Partition) bool {
	return *left == *right
}

func (left *StorageLayout) Equal(right *StorageLayout) bool {
	if left == right {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	if len(left.BootDriveLayout) != len(right.BootDriveLayout) {
		return false
	}
	for index, leftPartition := range left.BootDriveLayout {
		if !leftPartition.Equal(&right.BootDriveLayout[index]) {
			return false
		}
	}
	if left.ExtraMountPointsBasename != right.ExtraMountPointsBasename {
		return false
	}
	if left.Encrypt != right.Encrypt {
		return false
	}
	if left.UseKexec != right.UseKexec {
		return false
	}
	return true
}
