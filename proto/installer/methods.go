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
