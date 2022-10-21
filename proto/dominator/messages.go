package dominator

import (
	"time"

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

type ForceDisruptiveUpdateRequest struct {
	Hostname string
}

type ForceDisruptiveUpdateResponse struct{}

type GetDefaultImageRequest struct{}

type GetDefaultImageResponse struct {
	ImageName string
}

type GetSubsConfigurationRequest struct{}

type GetSubsConfigurationResponse sub.Configuration

type GetInfoForSubsRequest struct {
	Hostnames       []string            // Empty: match all hostnames.
	StatusesToMatch []string            // Empty: match all statuses.
	TagsToMatch     map[string][]string // Empty: match all tags.
}

type GetInfoForSubsResponse struct {
	Error string
	Subs  []SubInfo
}

type ListSubsRequest struct {
	Hostnames       []string            // Empty: match all hostnames.
	StatusesToMatch []string            // Empty: match all statuses.
	TagsToMatch     map[string][]string // Empty: match all tags.
}

type ListSubsResponse struct {
	Error     string
	Hostnames []string
}

type SetDefaultImageRequest struct {
	ImageName string
}

type SetDefaultImageResponse struct{}

type SubInfo struct {
	Hostname            string
	LastNote            string              `json:",omitempty"`
	LastDisruptionState sub.DisruptionState `json:",omitempty"`
	LastScanDuration    time.Duration       `json:",omitempty"`
	LastSuccessfulImage string              `json:",omitempty"`
	LastSyncTime        time.Time           `json:",omitempty"`
	LastUpdateTime      time.Time           `json:",omitempty"`
	PlannedImage        string              `json:",omitempty"`
	RequiredImage       string              `json:",omitempty"`
	StartTime           time.Time           `json:",omitempty"`
	Status              string
	SystemUptime        *time.Duration `json:",omitempty"`
}
