package dominator

import (
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

type ClearSafetyShutoffRequest struct {
	Hostname string
}

type ClearSafetyShutoffResponse struct{}

type ConfigureSubsRequest sub.Configuration

type ConfigureSubsResponse struct{}

type DisableUpdatesRequest struct {
	Reason string
}

type DisableUpdatesResponse struct{}

type EnableUpdatesRequest struct {
	Reason string
}

type EnableUpdatesResponse struct{}

type GetDefaultImageRequest struct{}

type GetDefaultImageResponse struct {
	ImageName string
}

type GetSubsConfigurationRequest struct{}

type GetSubsConfigurationResponse sub.Configuration

type ListSubsRequest struct {
	StatusToMatch string
}

type ListSubsResponse struct {
	Error     string
	Hostnames []string
}

type SetDefaultImageRequest struct {
	ImageName string
}

type SetDefaultImageResponse struct{}
