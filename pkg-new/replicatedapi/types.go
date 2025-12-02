package replicatedapi

import "time"

// PendingReleasesResponse represents the response from the /release/{appSlug}/pending API endpoint
type PendingReleasesResponse struct {
	ChannelReleases []ChannelRelease `json:"channelReleases"`
}

// SortOrder represents the order in which to sort releases
type SortOrder string

const SortOrderAscending SortOrder = "asc"
const SortOrderDescending SortOrder = "desc"

// PendingReleasesOptions represents options for fetching pending releases
type PendingReleasesOptions struct {
	IsSemverSupported      bool
	SortOrder              SortOrder
	CurrentChannelSequence int64
}

// ChannelRelease represents a single release in a channel
type ChannelRelease struct {
	ChannelID       string    `json:"channelId"`
	ChannelSequence int64     `json:"channelSequence"`
	ReleaseSequence int64     `json:"releaseSequence"`
	VersionLabel    string    `json:"versionLabel"`
	IsRequired      bool      `json:"isRequired"`
	CreatedAt       time.Time `json:"createdAt"`
}
