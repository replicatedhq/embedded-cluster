package dryrun

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

var _ replicatedapi.Client = (*ReplicatedAPIClient)(nil)

// ReplicatedAPIClient is a mockable implementation of the replicatedapi.Client interface.
type ReplicatedAPIClient struct {
	License      *licensewrapper.LicenseWrapper
	LicenseBytes []byte
	PendingReleases []replicatedapi.ChannelRelease
}

// SyncLicense returns the mocked license data.
func (c *ReplicatedAPIClient) SyncLicense(ctx context.Context) (*licensewrapper.LicenseWrapper, []byte, error) {
	// If License is not set but LicenseBytes is, parse the license from bytes
	if c.License == nil && len(c.LicenseBytes) > 0 {
		license, err := licensewrapper.LoadLicenseFromBytes(c.LicenseBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse license from bytes: %w", err)
		}
		c.License = &license
	}

	return c.License, c.LicenseBytes, nil
}

// GetPendingReleases returns the mocked pending releases data.
func (c *ReplicatedAPIClient) GetPendingReleases(ctx context.Context, channelID string, currentSequence int64, opts *replicatedapi.PendingReleasesOptions) (*replicatedapi.PendingReleasesResponse, error) {
	return &replicatedapi.PendingReleasesResponse{ChannelReleases: c.PendingReleases}, nil
}
