package replicatedapi

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// Ensure MockClient implements Client interface
var _ Client = (*MockClient)(nil)

// MockClient is a mock implementation of the Client interface for testing
type MockClient struct {
	SyncLicenseFunc        func(ctx context.Context, reportingInfo *ReportingInfo) (*kotsv1beta1.License, []byte, error)
	GetPendingReleasesFunc func(ctx context.Context, channelID string, currentSequence int64, opts *PendingReleasesOptions) (*PendingReleasesResponse, error)
}

// SyncLicense implements Client.SyncLicense
func (m *MockClient) SyncLicense(ctx context.Context, reportingInfo *ReportingInfo) (*kotsv1beta1.License, []byte, error) {
	if m.SyncLicenseFunc != nil {
		return m.SyncLicenseFunc(ctx, reportingInfo)
	}
	return nil, nil, nil
}

// GetPendingReleases implements Client.GetPendingReleases
func (m *MockClient) GetPendingReleases(ctx context.Context, channelID string, currentSequence int64, opts *PendingReleasesOptions) (*PendingReleasesResponse, error) {
	if m.GetPendingReleasesFunc != nil {
		return m.GetPendingReleasesFunc(ctx, channelID, currentSequence, opts)
	}
	return &PendingReleasesResponse{}, nil
}
