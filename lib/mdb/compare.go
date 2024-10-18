package mdb

import (
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

func (left Machine) compare(right Machine) bool {
	if left.Hostname != right.Hostname {
		return false
	}
	if left.DataSourceIdentifier != right.DataSourceIdentifier {
		return false
	}
	if left.DataSourceType != right.DataSourceType {
		return false
	}
	if left.IpAddress != right.IpAddress {
		return false
	}
	if left.Location != right.Location {
		return false
	}
	if left.RequiredImage != right.RequiredImage {
		return false
	}
	if left.PlannedImage != right.PlannedImage {
		return false
	}
	if left.DisableUpdates != right.DisableUpdates {
		return false
	}
	if left.OwnerGroup != right.OwnerGroup {
		return false
	}
	if !compareOwners(left.OwnerGroups, right.OwnerGroups) {
		return false
	}
	if !compareOwners(left.OwnerUsers, right.OwnerUsers) {
		return false
	}
	if !compareTags(left.Tags, right.Tags) {
		return false
	}
	if right.AwsMetadata == nil {
		if left.AwsMetadata != nil {
			return false
		}
	} else if left.AwsMetadata == nil {
		return false
	} else if !compareAwsMetadata(left.AwsMetadata, right.AwsMetadata) {
		return false
	}
	return true
}

func compareAwsMetadata(left, right *AwsMetadata) bool {
	if left.AccountId != right.AccountId {
		return false
	}
	if left.AccountName != right.AccountName {
		return false
	}
	if left.InstanceId != right.InstanceId {
		return false
	}
	if left.Region != right.Region {
		return false
	}
	if len(left.Tags) != len(right.Tags) {
		return false
	}
	return compareTags(left.Tags, right.Tags)
}

func compareOwners(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index, value := range left {
		if right[index] != value {
			return false
		}
	}
	return true
}

func compareTags(left, right tags.Tags) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
