package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test helpers

func newTestLicense(isSemverRequired bool) *kotsv1beta1.License {
	return &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug:          "test-app",
			LicenseID:        "test-license-id",
			IsSemverRequired: isSemverRequired,
			ChannelID:        "test-channel-123",
			ChannelName:      "Stable",
			LicenseSequence:  1,
			Channels: []kotsv1beta1.Channel{
				{
					ChannelID:   "test-channel-123",
					ChannelName: "Stable",
				},
			},
		},
	}
}

func newTestAirgapMetadata(requiredReleases []string) *airgap.AirgapMetadata {
	var releases []kotsv1beta1.AirgapReleaseMeta
	for _, versionLabel := range requiredReleases {
		releases = append(releases, kotsv1beta1.AirgapReleaseMeta{
			VersionLabel: versionLabel,
		})
	}

	return &airgap.AirgapMetadata{
		AirgapInfo: &kotsv1beta1.Airgap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "embedded-cluster.replicated.com/v1beta1",
				Kind:       "AirgapInfo",
			},
			Spec: kotsv1beta1.AirgapSpec{
				RequiredReleases: releases,
			},
		},
	}
}

// Tests

func TestValidateIsReleaseUpgradable_AppVersionDowngrade(t *testing.T) {
	tests := []struct {
		name               string
		currentAppVersion  string
		currentAppSequence int64
		targetAppVersion   string
		targetAppSequence  int64
		isSemverRequired   bool
		wantErr            bool
		wantValidationErr  bool
		wantErrContains    string
	}{
		{
			name:               "app version downgrade with semver - returns ValidationError",
			currentAppVersion:  "2.0.0",
			currentAppSequence: 100,
			targetAppVersion:   "1.9.0",
			targetAppSequence:  90,
			isSemverRequired:   true,
			wantErr:            true,
			wantValidationErr:  true,
			wantErrContains:    "downgrade detected",
		},
		{
			name:               "app version downgrade with sequence - returns ValidationError",
			currentAppVersion:  "Release 100",
			currentAppSequence: 100,
			targetAppVersion:   "Release 50",
			targetAppSequence:  50,
			isSemverRequired:   false,
			wantErr:            true,
			wantValidationErr:  true,
			wantErrContains:    "downgrade detected",
		},
		{
			name:               "app version upgrade with semver - succeeds",
			currentAppVersion:  "1.9.0",
			currentAppSequence: 90,
			targetAppVersion:   "2.0.0",
			targetAppSequence:  100,
			isSemverRequired:   true,
			wantErr:            false,
			wantValidationErr:  false,
		},
		{
			name:               "app version same version - succeeds",
			currentAppVersion:  "2.0.0",
			currentAppSequence: 100,
			targetAppVersion:   "2.0.0",
			targetAppSequence:  100,
			isSemverRequired:   true,
			wantErr:            false,
			wantValidationErr:  false,
		},
		{
			name:               "app version invalid semver - returns internal error",
			currentAppVersion:  "invalid-version",
			currentAppSequence: 100,
			targetAppVersion:   "2.0.0",
			targetAppSequence:  100,
			isSemverRequired:   true,
			wantErr:            true,
			wantValidationErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts := UpgradableOptions{
				IsAirgap:           true, // Use airgap to skip required releases check
				CurrentAppVersion:  tt.currentAppVersion,
				CurrentAppSequence: tt.currentAppSequence,
				TargetAppVersion:   tt.targetAppVersion,
				TargetAppSequence:  tt.targetAppSequence,
				CurrentECVersion:   "2.10.0+k8s-1.31",
				TargetECVersion:    "2.11.0+k8s-1.32",
				License:            newTestLicense(tt.isSemverRequired),
				AirgapMetadata:     newTestAirgapMetadata(nil), // No required releases
			}

			err := ValidateIsReleaseUpgradable(context.Background(), opts)

			if tt.wantErr {
				req.Error(err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				if tt.wantValidationErr {
					assert.True(t, IsValidationError(err), "expected ValidationError")
				} else {
					assert.False(t, IsValidationError(err), "expected internal error, not ValidationError")
				}
			} else {
				req.NoError(err)
			}
		})
	}
}

func TestValidateIsReleaseUpgradable_RequiredReleases(t *testing.T) {
	tests := []struct {
		name               string
		isAirgap           bool
		airgapMetadata     *airgap.AirgapMetadata
		replicatedAPI      replicatedapi.Client
		currentAppSequence int64
		targetAppSequence  int64
		wantErr            bool
		wantValidationErr  bool
		wantErrContains    string
	}{
		{
			name:               "required releases in airgap mode - returns ValidationError",
			isAirgap:           true,
			airgapMetadata:     newTestAirgapMetadata([]string{"1.1.0", "1.2.0"}),
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            true,
			wantValidationErr:  true,
			wantErrContains:    "intermediate version(s)",
		},
		{
			name:     "required releases in online mode - returns ValidationError",
			isAirgap: false,
			replicatedAPI: &replicatedapi.MockClient{
				GetPendingReleasesFunc: func(ctx context.Context, channelID string, currentSequence int64, opts *replicatedapi.PendingReleasesOptions) (*replicatedapi.PendingReleasesResponse, error) {
					return &replicatedapi.PendingReleasesResponse{
						ChannelReleases: []replicatedapi.ChannelRelease{
							{
								ChannelSequence: 11,
								VersionLabel:    "1.1.0",
								IsRequired:      false,
							},
							{
								ChannelSequence: 12,
								VersionLabel:    "1.2.0",
								IsRequired:      true,
							},
							{
								ChannelSequence: 13,
								VersionLabel:    "1.3.0",
								IsRequired:      false,
							},
						},
					}, nil
				},
			},
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            true,
			wantValidationErr:  true,
			wantErrContains:    "1.2.0",
		},
		{
			name:               "no required releases in airgap mode - succeeds",
			isAirgap:           true,
			airgapMetadata:     newTestAirgapMetadata(nil),
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            false,
			wantValidationErr:  false,
		},
		{
			name:     "no required releases in online mode - succeeds",
			isAirgap: false,
			replicatedAPI: &replicatedapi.MockClient{
				GetPendingReleasesFunc: func(ctx context.Context, channelID string, currentSequence int64, opts *replicatedapi.PendingReleasesOptions) (*replicatedapi.PendingReleasesResponse, error) {
					return &replicatedapi.PendingReleasesResponse{
						ChannelReleases: []replicatedapi.ChannelRelease{
							{
								ChannelSequence: 11,
								VersionLabel:    "1.1.0",
								IsRequired:      false,
							},
							{
								ChannelSequence: 12,
								VersionLabel:    "1.2.0",
								IsRequired:      false,
							},
						},
					}, nil
				},
			},
			currentAppSequence: 10,
			targetAppSequence:  12,
			wantErr:            false,
			wantValidationErr:  false,
		},
		{
			name:               "required releases missing airgap metadata - returns internal error",
			isAirgap:           true,
			airgapMetadata:     nil,
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            true,
			wantValidationErr:  false,
			wantErrContains:    "airgap metadata is required",
		},
		{
			name:               "required releases missing API client - returns internal error",
			isAirgap:           false,
			replicatedAPI:      nil,
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            true,
			wantValidationErr:  false,
			wantErrContains:    "replicated API client is required",
		},
		{
			name:     "required releases API error - returns internal error",
			isAirgap: false,
			replicatedAPI: &replicatedapi.MockClient{
				GetPendingReleasesFunc: func(ctx context.Context, channelID string, currentSequence int64, opts *replicatedapi.PendingReleasesOptions) (*replicatedapi.PendingReleasesResponse, error) {
					return nil, fmt.Errorf("API connection failed")
				},
			},
			currentAppSequence: 10,
			targetAppSequence:  13,
			wantErr:            true,
			wantValidationErr:  false,
			wantErrContains:    "failed to fetch pending releases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts := UpgradableOptions{
				IsAirgap:           tt.isAirgap,
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: tt.currentAppSequence,
				TargetAppVersion:   "1.3.0",
				TargetAppSequence:  tt.targetAppSequence,
				CurrentECVersion:   "2.10.0+k8s-1.31",
				TargetECVersion:    "2.11.0+k8s-1.32",
				License:            newTestLicense(true),
				ChannelID:          "test-channel-123",
				AirgapMetadata:     tt.airgapMetadata,
				ReplicatedAPI:      tt.replicatedAPI,
			}

			err := ValidateIsReleaseUpgradable(context.Background(), opts)

			if tt.wantErr {
				req.Error(err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				if tt.wantValidationErr {
					assert.True(t, IsValidationError(err), "expected ValidationError")
				} else {
					assert.False(t, IsValidationError(err), "expected internal error, not ValidationError")
				}
			} else {
				req.NoError(err)
			}
		})
	}
}

func TestValidateIsReleaseUpgradable_ECVersionDowngrade(t *testing.T) {
	tests := []struct {
		name              string
		currentECVersion  string
		targetECVersion   string
		wantErr           bool
		wantValidationErr bool
		wantErrContains   string
	}{
		{
			name:              "EC version downgrade - returns ValidationError",
			currentECVersion:  "2.11.0+k8s-1.32",
			targetECVersion:   "2.10.0+k8s-1.31",
			wantErr:           true,
			wantValidationErr: true,
			wantErrContains:   "Embedded Cluster version",
		},
		{
			name:              "EC version upgrade - succeeds",
			currentECVersion:  "2.10.0+k8s-1.31",
			targetECVersion:   "2.11.0+k8s-1.32",
			wantErr:           false,
			wantValidationErr: false,
		},
		{
			name:              "EC version same - succeeds",
			currentECVersion:  "2.10.0+k8s-1.31",
			targetECVersion:   "2.10.0+k8s-1.31",
			wantErr:           false,
			wantValidationErr: false,
		},
		{
			name:              "EC version invalid format - returns internal error",
			currentECVersion:  "invalid",
			targetECVersion:   "2.10.0+k8s-1.31",
			wantErr:           true,
			wantValidationErr: false,
			wantErrContains:   "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts := UpgradableOptions{
				IsAirgap:           true,
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 10,
				TargetAppVersion:   "2.0.0",
				TargetAppSequence:  20,
				CurrentECVersion:   tt.currentECVersion,
				TargetECVersion:    tt.targetECVersion,
				License:            newTestLicense(true),
				AirgapMetadata:     newTestAirgapMetadata(nil),
			}

			err := ValidateIsReleaseUpgradable(context.Background(), opts)

			if tt.wantErr {
				req.Error(err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				if tt.wantValidationErr {
					assert.True(t, IsValidationError(err), "expected ValidationError")
				} else {
					assert.False(t, IsValidationError(err), "expected internal error, not ValidationError")
				}
			} else {
				req.NoError(err)
			}
		})
	}
}

func TestValidateIsReleaseUpgradable_K8sVersionSkip(t *testing.T) {
	tests := []struct {
		name              string
		currentECVersion  string
		targetECVersion   string
		wantErr           bool
		wantValidationErr bool
		wantErrContains   string
	}{
		{
			name:              "k8s minor version skip - returns ValidationError",
			currentECVersion:  "2.10.0+k8s-1.30",
			targetECVersion:   "2.12.0+k8s-1.32",
			wantErr:           true,
			wantValidationErr: true,
			wantErrContains:   "skip detected",
		},
		{
			name:              "k8s minor version increment by one - succeeds",
			currentECVersion:  "2.10.0+k8s-1.31",
			targetECVersion:   "2.11.0+k8s-1.32",
			wantErr:           false,
			wantValidationErr: false,
		},
		{
			name:              "k8s same version - succeeds",
			currentECVersion:  "2.10.0+k8s-1.32",
			targetECVersion:   "2.11.0+k8s-1.32",
			wantErr:           false,
			wantValidationErr: false,
		},
		{
			name:              "k8s version extraction fails - returns internal error",
			currentECVersion:  "2.10.0+invalid-format",
			targetECVersion:   "2.11.0+k8s-1.32",
			wantErr:           true,
			wantValidationErr: false,
			wantErrContains:   "failed to extract k8s version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts := UpgradableOptions{
				IsAirgap:           true,
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 10,
				TargetAppVersion:   "2.0.0",
				TargetAppSequence:  20,
				CurrentECVersion:   tt.currentECVersion,
				TargetECVersion:    tt.targetECVersion,
				License:            newTestLicense(true),
				AirgapMetadata:     newTestAirgapMetadata(nil),
			}

			err := ValidateIsReleaseUpgradable(context.Background(), opts)

			if tt.wantErr {
				req.Error(err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				if tt.wantValidationErr {
					assert.True(t, IsValidationError(err), "expected ValidationError")
				} else {
					assert.False(t, IsValidationError(err), "expected internal error, not ValidationError")
				}
			} else {
				req.NoError(err)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ValidationError from NewRequiredReleasesError",
			err:      NewRequiredReleasesError([]string{"1.0.0"}, "2.0.0"),
			expected: true,
		},
		{
			name:     "ValidationError from NewAppVersionDowngradeError",
			err:      NewAppVersionDowngradeError("2.0.0", "1.0.0"),
			expected: true,
		},
		{
			name:     "ValidationError from NewECVersionDowngradeError",
			err:      NewECVersionDowngradeError("2.11.0", "2.10.0"),
			expected: true,
		},
		{
			name:     "ValidationError from NewK8sVersionSkipError",
			err:      NewK8sVersionSkipError("1.30", "1.32"),
			expected: true,
		},
		{
			name:     "regular error is not ValidationError",
			err:      fmt.Errorf("some internal error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidationError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
