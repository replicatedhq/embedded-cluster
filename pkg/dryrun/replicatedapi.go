package dryrun

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"sigs.k8s.io/yaml"
)

var _ replicatedapi.Client = (*ReplicatedAPIClient)(nil)

// ReplicatedAPIClient is a mockable implementation of the replicatedapi.Client interface.
type ReplicatedAPIClient struct {
	License         *kotsv1beta1.License
	LicenseBytes    []byte
	PendingReleases []replicatedapi.ChannelRelease
}

// SyncLicense returns the mocked license data.
func (c *ReplicatedAPIClient) SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error) {
	// If License is not set but LicenseBytes is, parse the license from bytes
	if c.License == nil && len(c.LicenseBytes) > 0 {
		var license kotsv1beta1.License
		if err := yaml.Unmarshal(c.LicenseBytes, &license); err != nil {
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
