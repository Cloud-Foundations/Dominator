package amipublisher

import (
	"errors"
	"github.com/Symantec/Dominator/lib/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"path"
	"strings"
	"time"
)

func attachVolume(awsService *ec2.EC2, instance *ec2.Instance, volumeId string,
	logger log.Logger) error {
	usedBlockDevices := make(map[string]struct{})
	for _, device := range instance.BlockDeviceMappings {
		usedBlockDevices[aws.StringValue(device.DeviceName)] = struct{}{}
	}
	var blockDeviceName string
	for c := 'f'; c <= 'p'; c++ {
		name := "/dev/sd" + string(c)
		if _, ok := usedBlockDevices[name]; !ok {
			blockDeviceName = name
			break
		}
	}
	if blockDeviceName == "" {
		return errors.New("no space for new block device")
	}
	_, err := awsService.AttachVolume(&ec2.AttachVolumeInput{
		Device:     aws.String(blockDeviceName),
		InstanceId: instance.InstanceId,
		VolumeId:   aws.String(volumeId),
	})
	if err != nil {
		return err
	}
	blockDevMappings := make([]*ec2.InstanceBlockDeviceMappingSpecification, 1)
	blockDevMappings[0] = &ec2.InstanceBlockDeviceMappingSpecification{
		DeviceName: aws.String(blockDeviceName),
		Ebs: &ec2.EbsInstanceBlockDeviceSpecification{
			DeleteOnTermination: aws.Bool(true),
			VolumeId:            aws.String(volumeId),
		},
	}
	_, err = awsService.ModifyInstanceAttribute(
		&ec2.ModifyInstanceAttributeInput{
			BlockDeviceMappings: blockDevMappings,
			InstanceId:          instance.InstanceId,
		})
	if err != nil {
		return err
	}
	logger.Printf("Requested attach(%s): %s on %s, waiting...\n",
		aws.StringValue(instance.InstanceId), volumeId, blockDeviceName)
	volumeIds := make([]string, 1)
	volumeIds[0] = volumeId
	for ; true; time.Sleep(time.Second) {
		desc, err := awsService.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: aws.StringSlice(volumeIds),
		})
		if err != nil {
			return err
		}
		state := *desc.Volumes[0].Attachments[0].State
		logger.Printf("state: \"%s\"\n", state)
		if state == ec2.VolumeAttachmentStateAttached {
			break
		}
	}
	logger.Printf("Attached: %s\n", volumeId)
	return nil
}

func createSnapshot(awsService *ec2.EC2, volumeId string,
	description string, tags map[string]string, logger log.Logger) (
	string, error) {
	snapshot, err := awsService.CreateSnapshot(&ec2.CreateSnapshotInput{
		VolumeId:    aws.String(volumeId),
		Description: aws.String(description),
	})
	if err != nil {
		return "", err
	}
	snapshotIds := make([]string, 1)
	snapshotIds[0] = *snapshot.SnapshotId
	logger.Printf("Created: %s\n", *snapshot.SnapshotId)
	// Strip out possible Name tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = description
	tags = newTags
	if err := createTags(awsService, *snapshot.SnapshotId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *snapshot.SnapshotId)
	err = awsService.WaitUntilSnapshotCompleted(&ec2.DescribeSnapshotsInput{
		SnapshotIds: aws.StringSlice(snapshotIds),
	})
	if err != nil {
		return "", err
	}
	return *snapshot.SnapshotId, nil
}

func createTags(awsService *ec2.EC2, resourceId string,
	tags map[string]string) error {
	resourceIds := make([]string, 1)
	resourceIds[0] = resourceId
	awsTags := make([]*ec2.Tag, 0, len(tags))
	for key, value := range tags {
		awsTags = append(awsTags,
			&ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	_, err := awsService.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice(resourceIds),
		Tags:      awsTags,
	})
	return err
}

func createVolume(awsService *ec2.EC2, availabilityZone *string, size uint64,
	tags map[string]string, logger log.Logger) (string, error) {
	// Strip out possible ExpiresAt tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "ExpiresAt":
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = "image unpacker"
	tags = newTags
	sizeInGiB := int64(size) >> 30
	if sizeInGiB<<30 < int64(size) {
		sizeInGiB++
	}
	volume, err := awsService.CreateVolume(&ec2.CreateVolumeInput{
		AvailabilityZone: availabilityZone,
		Encrypted:        aws.Bool(true),
		Size:             aws.Int64(sizeInGiB),
		VolumeType:       aws.String("gp2"),
	})
	if err != nil {
		return "", err
	}
	volumeIds := make([]string, 1)
	volumeIds[0] = *volume.VolumeId
	logger.Printf("Created: %s\n", *volume.VolumeId)
	if err := createTags(awsService, *volume.VolumeId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *volume.VolumeId)
	err = awsService.WaitUntilVolumeAvailable(&ec2.DescribeVolumesInput{
		VolumeIds: aws.StringSlice(volumeIds),
	})
	if err != nil {
		return "", err
	}
	return *volume.VolumeId, nil
}

func deleteSnapshot(awsService *ec2.EC2, snapshotId string) error {
	for i := 0; i < 5; i++ {
		_, err := awsService.DeleteSnapshot(&ec2.DeleteSnapshotInput{
			SnapshotId: aws.String(snapshotId),
		})
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "in use by ami") {
			return err
		}
		time.Sleep(time.Second)
	}
	return errors.New("timed out waiting for delete: " + snapshotId)
}

func deleteTagsFromResources(awsService *ec2.EC2, tagKeys []string,
	resourceId ...string) error {
	if len(tagKeys) < 1 {
		return nil
	}
	resourceIds := make([]string, 0)
	for _, id := range resourceId {
		if id != "" {
			resourceIds = append(resourceIds, id)
		}
	}
	if len(resourceIds) < 1 {
		return nil
	}
	tags := make([]*ec2.Tag, 0, len(tagKeys))
	for _, tagKey := range tagKeys {
		tags = append(tags, &ec2.Tag{Key: aws.String(tagKey)})
	}
	_, err := awsService.DeleteTags(&ec2.DeleteTagsInput{
		Resources: aws.StringSlice(resourceIds),
		Tags:      tags,
	})
	return err
}

func deleteVolume(awsService *ec2.EC2, volumeId string) error {
	_, err := awsService.DeleteVolume(&ec2.DeleteVolumeInput{
		VolumeId: aws.String(volumeId),
	})
	return err
}

func deregisterAmi(awsService *ec2.EC2, amiId string) error {
	_, err := awsService.DeregisterImage(&ec2.DeregisterImageInput{
		ImageId: aws.String(amiId),
	})
	if err != nil {
		return err
	}
	imageIds := make([]*string, 1)
	imageIds[0] = aws.String(amiId)
	for i := 0; i < 60; i++ {
		out, err := awsService.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: imageIds,
		})
		if err != nil {
			return err
		}
		if len(out.Images) < 1 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("timed out waiting for deregister: " + amiId)
}

func getImages(awsService *ec2.EC2, nameTag string, tagKey string) (
	[]*ec2.Image, error) {
	filters := make([]*ec2.Filter, 1, 2)
	filters[0] = &ec2.Filter{
		Name:   aws.String("tag:Name"),
		Values: aws.StringSlice([]string{nameTag}),
	}
	if tagKey != "" {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("tag-key"),
			Values: aws.StringSlice([]string{tagKey}),
		})
	}
	out, err := awsService.DescribeImages(
		&ec2.DescribeImagesInput{Filters: filters})
	if err != nil {
		return nil, err
	}
	return out.Images, nil
}

func getInstances(awsService *ec2.EC2, nameTag string) (
	[]*ec2.Instance, error) {
	if nameTag == "" {
		return nil, errors.New("no name given")
	}
	tagValues := make([]string, 1)
	tagValues[0] = nameTag
	filters := make([]*ec2.Filter, 1)
	filters[0] = &ec2.Filter{
		Name:   aws.String("tag:Name"),
		Values: aws.StringSlice(tagValues),
	}
	out, err := awsService.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	instances := make([]*ec2.Instance, 0)
	for _, reservation := range out.Reservations {
		for _, instance := range reservation.Instances {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

func getInstanceIds(instances []*ec2.Instance) []string {
	instanceIds := make([]string, 0, len(instances))
	for _, instance := range instances {
		instanceIds = append(instanceIds, aws.StringValue(instance.InstanceId))
	}
	return instanceIds
}

func getRunningInstance(awsService *ec2.EC2, instances []*ec2.Instance,
	logger log.Logger) (*ec2.Instance, error) {
	for _, instance := range instances {
		if aws.StringValue(instance.State.Name) ==
			ec2.InstanceStateNameRunning {
			return instance, nil
		}
	}
	var stoppedInstance *ec2.Instance
	isStopped := false
	for _, instance := range instances {
		if stoppedInstance != nil {
			break
		}
		switch aws.StringValue(instance.State.Name) {
		case ec2.InstanceStateNameStopped:
			stoppedInstance = instance
			isStopped = true
		case ec2.InstanceStateNamePending:
			stoppedInstance = instance
		}
	}
	if stoppedInstance == nil {
		return nil, nil
	}
	instanceIds := make([]*string, 1)
	instanceIds[0] = stoppedInstance.InstanceId
	if isStopped {
		logger.Printf("starting instance: %s\n",
			aws.StringValue(instanceIds[0]))
		_, err := awsService.StartInstances(&ec2.StartInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			return nil, err
		}
		stoppedInstance.LaunchTime = aws.Time(time.Now())
	}
	logger.Printf("waiting for pending instance: %s\n",
		aws.StringValue(instanceIds[0]))
	err := awsService.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return nil, err
	}
	return stoppedInstance, nil
}

func getSecurityGroup(awsService *ec2.EC2, tagKey string) (
	*ec2.SecurityGroup, error) {
	out, err := awsService.DescribeSecurityGroups(
		&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{&ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: aws.StringSlice([]string{tagKey}),
			}}})
	if err != nil {
		return nil, err
	}
	if len(out.SecurityGroups) > 1 {
		return nil, errors.New("too many security groups found")
	}
	return out.SecurityGroups[0], nil
}

func getSubnet(awsService *ec2.EC2, vpcId string) (*ec2.Subnet, error) {
	out, err := awsService.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{&ec2.Filter{
			Name:   aws.String("vpc-id"),
			Values: aws.StringSlice([]string{vpcId}),
		}}})
	if err != nil {
		return nil, err
	}
	if len(out.Subnets) < 1 {
		return nil, errors.New("no subnets found")
	}
	for _, subnet := range out.Subnets {
		if aws.Int64Value(subnet.AvailableIpAddressCount) > 0 {
			return subnet, nil
		}
	}
	return nil, errors.New("no subnets with available IPs found")
}

func getVpc(awsService *ec2.EC2, tagKey string) (*ec2.Vpc, error) {
	out, err := awsService.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{&ec2.Filter{
			Name:   aws.String("tag-key"),
			Values: aws.StringSlice([]string{tagKey}),
		}}})
	if err != nil {
		return nil, err
	}
	if len(out.Vpcs) < 1 {
		return nil, errors.New("no VPCs found")
	}
	if len(out.Vpcs) > 1 {
		return nil, errors.New("too many VPCs found")
	}
	return out.Vpcs[0], nil
}

func registerAmi(awsService *ec2.EC2, snapshotId string, amiName string,
	imageName string, tags map[string]string, logger log.Logger) (
	string, error) {
	rootDevName := "/dev/sda1"
	blkDevMaps := make([]*ec2.BlockDeviceMapping, 1)
	blkDevMaps[0] = &ec2.BlockDeviceMapping{
		DeviceName: aws.String(rootDevName),
		Ebs: &ec2.EbsBlockDevice{
			DeleteOnTermination: aws.Bool(true),
			SnapshotId:          aws.String(snapshotId),
			VolumeType:          aws.String("gp2"),
		},
	}
	if amiName == "" {
		amiName = imageName
	}
	amiName = strings.Replace(amiName, ":", ".", -1)
	ami, err := awsService.RegisterImage(&ec2.RegisterImageInput{
		Architecture:        aws.String("x86_64"),
		BlockDeviceMappings: blkDevMaps,
		Description:         aws.String(imageName),
		Name:                aws.String(amiName),
		RootDeviceName:      aws.String(rootDevName),
		SriovNetSupport:     aws.String("simple"),
		VirtualizationType:  aws.String("hvm"),
	})
	if err != nil {
		return "", err
	}
	logger.Printf("Created: %s\n", *ami.ImageId)
	imageIds := make([]string, 1)
	imageIds[0] = *ami.ImageId
	// Strip out possible Name tag.
	newTags := make(map[string]string)
	for key, value := range tags {
		switch key {
		case "Name":
		default:
			newTags[key] = value
		}
	}
	newTags["Name"] = path.Dir(imageName)
	tags = newTags
	if err := createTags(awsService, *ami.ImageId, tags); err != nil {
		return "", err
	}
	logger.Printf("Tagged: %s, waiting...\n", *ami.ImageId)
	err = awsService.WaitUntilImageAvailable(&ec2.DescribeImagesInput{
		ImageIds: aws.StringSlice(imageIds),
	})
	if err != nil {
		return "", err
	}
	return *ami.ImageId, nil
}

func stopInstances(awsService *ec2.EC2, instanceIds ...string) error {
	_, err := awsService.StopInstances(&ec2.StopInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	return err
}

func libTerminateInstances(awsService *ec2.EC2, instanceIds ...string) error {
	_, err := awsService.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	return err
}
