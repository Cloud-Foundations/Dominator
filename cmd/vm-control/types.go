package main

import (
	"bytes"
	"strings"

	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type volumeTypeList []hyper_proto.VolumeType

func (vtl *volumeTypeList) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(`"`)
	for index, vtype := range *vtl {
		buffer.WriteString(vtype.String())
		if index < len(*vtl)-1 {
			buffer.WriteString(",")
		}
	}
	buffer.WriteString(`"`)
	return buffer.String()
}

func (vtl *volumeTypeList) Set(value string) error {
	newList := make(volumeTypeList, 0)
	if value != "" {
		vtypeStrings := strings.Split(value, ",")
		for _, vtypeString := range vtypeStrings {
			var vtype hyper_proto.VolumeType
			if err := vtype.Set(vtypeString); err != nil {
				return err
			}
			newList = append(newList, vtype)
		}
	}
	*vtl = newList
	return nil
}
