package mdb

func (dest *Machine) updateFrom(source Machine) {
	if dest.Hostname != source.Hostname {
		return
	}
	if source.IpAddress != "" {
		dest.IpAddress = source.IpAddress
	}
	if source.Location != "" {
		dest.Location = source.Location
	}
	if source.RequiredImage != "" {
		dest.RequiredImage = source.RequiredImage
		dest.DisableUpdates = source.DisableUpdates
	}
	if source.PlannedImage != "" {
		dest.PlannedImage = source.PlannedImage
	}
	if source.OwnerGroup != "" {
		dest.OwnerGroup = source.OwnerGroup
	}
	if source.OwnerGroups != nil {
		dest.OwnerGroups = source.OwnerGroups
	}
	if source.OwnerUsers != nil {
		dest.OwnerUsers = source.OwnerUsers
	}
	if source.Tags != nil {
		dest.Tags = source.Tags
	}
	if source.AwsMetadata != nil {
		if dest.AwsMetadata == nil {
			dest.AwsMetadata = source.AwsMetadata
		} else if !compareAwsMetadata(dest.AwsMetadata, source.AwsMetadata) {
			dest.AwsMetadata = source.AwsMetadata
		}
	}
}
