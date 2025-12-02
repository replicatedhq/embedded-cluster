package validation

import (
	"errors"
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

type airgapReleaseData struct {
	versionLabel string
	updateCursor string
}

func newTestAirgapMetadataWithSequences(releases []airgapReleaseData) *airgap.AirgapMetadata {
	var releaseMetas []kotsv1beta1.AirgapReleaseMeta
	for _, r := range releases {
		releaseMetas = append(releaseMetas, kotsv1beta1.AirgapReleaseMeta{
			VersionLabel: r.versionLabel,
			UpdateCursor: r.updateCursor,
		})
	}

	return &airgap.AirgapMetadata{
		AirgapInfo: &kotsv1beta1.Airgap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "embedded-cluster.replicated.com/v1beta1",
				Kind:       "AirgapInfo",
			},
			Spec: kotsv1beta1.AirgapSpec{
				RequiredReleases: releaseMetas,
			},
		},
	}
}

// Tests

func TestWithAirgapRequiredReleases(t *testing.T) {
	tests := []struct {
		name                             string
		metadata                         *airgap.AirgapMetadata
		currentSequence                  int64
		expectedReleases                 []string
		expectedCurrentReleaseIsRequired bool
		expectError                      bool
		errorContains                    string
	}{
		{
			name:             "no required releases",
			metadata:         newTestAirgapMetadataWithSequences([]airgapReleaseData{}),
			currentSequence:  100,
			expectedReleases: []string{},
			expectError:      false,
		},
		{
			name: "all releases newer than current",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.5.0", updateCursor: "500"},
				{versionLabel: "1.4.0", updateCursor: "400"},
				{versionLabel: "1.3.0", updateCursor: "300"},
			}),
			currentSequence:  100,
			expectedReleases: []string{"1.5.0", "1.4.0", "1.3.0"},
			expectError:      false,
		},
		{
			name: "mixed releases - stops at older release",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.5.0", updateCursor: "500"},
				{versionLabel: "1.4.0", updateCursor: "400"},
				{versionLabel: "1.2.0", updateCursor: "200"},
				{versionLabel: "1.1.0", updateCursor: "100"},
			}),
			currentSequence:  300,
			expectedReleases: []string{"1.5.0", "1.4.0"},
			expectError:      false,
		},
		{
			name: "all releases older than current",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.2.0", updateCursor: "200"},
				{versionLabel: "1.1.0", updateCursor: "100"},
			}),
			currentSequence:  300,
			expectedReleases: []string{},
			expectError:      false,
		},
		{
			name: "current release is required",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.0.0", updateCursor: "300"},
			}),
			currentSequence:                  300,
			expectedReleases:                 []string{},
			expectError:                      false,
			expectedCurrentReleaseIsRequired: true,
		},
		{
			name: "current release is required and there's other required releases",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.4.0", updateCursor: "400"},
				{versionLabel: "1.3.0", updateCursor: "300"},
				{versionLabel: "1.2.0", updateCursor: "200"},
			}),
			currentSequence:                  300,
			expectedReleases:                 []string{"1.4.0"},
			expectedCurrentReleaseIsRequired: true,
			expectError:                      false,
		},
		{
			name:            "nil metadata",
			metadata:        nil,
			currentSequence: 100,
			expectError:     true,
			errorContains:   "airgap metadata is required",
		},
		{
			name: "nil airgap info",
			metadata: &airgap.AirgapMetadata{
				AirgapInfo: nil,
			},
			currentSequence: 100,
			expectError:     true,
			errorContains:   "airgap metadata is required",
		},
		{
			name: "invalid update cursor",
			metadata: newTestAirgapMetadataWithSequences([]airgapReleaseData{
				{versionLabel: "1.5.0", updateCursor: "invalid-number"},
			}),
			currentSequence: 100,
			expectError:     true,
			errorContains:   "failed to parse airgap spec required release update cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := UpgradableOptions{
				CurrentAppSequence: tt.currentSequence,
			}

			err := opts.WithAirgapRequiredReleases(tt.metadata)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if len(tt.expectedReleases) == 0 {
					assert.Empty(t, opts.requiredReleases)
				} else {
					assert.Equal(t, tt.expectedReleases, opts.requiredReleases)
				}
				assert.Equal(t, tt.expectedCurrentReleaseIsRequired, opts.currentReleaseIsRequired)
			}
		})
	}
}

func TestHandlePendingReleases(t *testing.T) {
	tests := []struct {
		name                             string
		pendingReleases                  []replicatedapi.ChannelRelease
		currentSequence                  int64
		targetSequence                   int64
		expectedReleases                 []string
		expectedCurrentReleaseIsRequired bool
	}{
		{
			name: "no required releases",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: false},
				{ChannelSequence: 102, VersionLabel: "1.2.0", IsRequired: false},
				{ChannelSequence: 103, VersionLabel: "1.3.0", IsRequired: false},
			},
			currentSequence:  100,
			targetSequence:   104,
			expectedReleases: []string{},
		},
		{
			name: "all releases required",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
				{ChannelSequence: 102, VersionLabel: "1.2.0", IsRequired: true},
				{ChannelSequence: 103, VersionLabel: "1.3.0", IsRequired: true},
			},
			currentSequence:  100,
			targetSequence:   104,
			expectedReleases: []string{"1.1.0", "1.2.0", "1.3.0"},
		},
		{
			name: "mixed required and not required",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
				{ChannelSequence: 102, VersionLabel: "1.2.0", IsRequired: false},
				{ChannelSequence: 103, VersionLabel: "1.3.0", IsRequired: true},
			},
			currentSequence:  100,
			targetSequence:   104,
			expectedReleases: []string{"1.1.0", "1.3.0"},
		},
		{
			name: "stops at target sequence",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
				{ChannelSequence: 102, VersionLabel: "1.2.0", IsRequired: true},
				{ChannelSequence: 103, VersionLabel: "1.3.0", IsRequired: true},
				{ChannelSequence: 104, VersionLabel: "1.4.0", IsRequired: true},
				{ChannelSequence: 105, VersionLabel: "1.5.0", IsRequired: true},
			},
			currentSequence:  100,
			targetSequence:   104,
			expectedReleases: []string{"1.1.0", "1.2.0", "1.3.0"},
		},
		{
			name:             "empty pending releases",
			pendingReleases:  []replicatedapi.ChannelRelease{},
			currentSequence:  100,
			targetSequence:   104,
			expectedReleases: []string{},
		},
		{
			name: "single required release",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
			},
			currentSequence:  100,
			targetSequence:   102,
			expectedReleases: []string{"1.1.0"},
		},
		{
			name: "target sequence equals first release - no releases collected",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 100, VersionLabel: "1.0.0", IsRequired: true},
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
			},
			currentSequence:  99,
			targetSequence:   100,
			expectedReleases: []string{},
		},
		{
			name: "current sequence is required",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 99, VersionLabel: "1.0.0", IsRequired: true},
			},
			currentSequence:                  99,
			targetSequence:                   100,
			expectedReleases:                 []string{},
			expectedCurrentReleaseIsRequired: true,
		},
		{
			name: "current sequence is required and there's other required releases",
			pendingReleases: []replicatedapi.ChannelRelease{
				{ChannelSequence: 99, VersionLabel: "1.0.0", IsRequired: true},
				{ChannelSequence: 101, VersionLabel: "1.1.0", IsRequired: true},
				{ChannelSequence: 102, VersionLabel: "1.2.0", IsRequired: false},
				{ChannelSequence: 103, VersionLabel: "1.3.0", IsRequired: true},
			},
			currentSequence:                  99,
			targetSequence:                   104,
			expectedReleases:                 []string{"1.1.0", "1.3.0"},
			expectedCurrentReleaseIsRequired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := UpgradableOptions{
				CurrentAppSequence: tt.currentSequence,
				TargetAppSequence:  tt.targetSequence,
			}

			opts.handlePendingReleases(tt.pendingReleases)

			if len(tt.expectedReleases) == 0 {
				assert.Empty(t, opts.requiredReleases)
			} else {
				assert.Equal(t, tt.expectedReleases, opts.requiredReleases)
			}
			assert.Equal(t, tt.expectedCurrentReleaseIsRequired, opts.currentReleaseIsRequired)
		})
	}
}

func TestValidateIsReleaseUpgradable(t *testing.T) {
	tests := []struct {
		name                string
		opts                UpgradableOptions
		expectError         bool
		expectValidationErr bool
	}{
		{
			name: "valid upgrade - semver",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "valid upgrade - sequence-based",
			opts: UpgradableOptions{
				CurrentAppVersion:  "v100",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "v101",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(false),
				requiredReleases:   []string{},
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "valid k8s minor version upgrade",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.30",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "valid upgrade - same versions",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.0.0",
				TargetAppSequence:  100,
				TargetECVersion:    "2.0.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "valid upgrade - patch version only",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.0.1",
				TargetAppSequence:  101,
				TargetECVersion:    "2.0.1+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "app version downgrade - semver",
			opts: UpgradableOptions{
				CurrentAppVersion:  "2.0.0",
				CurrentAppSequence: 200,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.5.0",
				TargetAppSequence:  150,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "app version downgrade - sequence-based",
			opts: UpgradableOptions{
				CurrentAppVersion:  "v200",
				CurrentAppSequence: 200,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "v150",
				TargetAppSequence:  150,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(false),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "required releases present",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.5.0",
				TargetAppSequence:  500,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{"1.1.0", "1.2.0"},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "current release failed and is required",
			opts: UpgradableOptions{
				CurrentAppVersion:        "1.0.0",
				CurrentAppSequence:       100,
				CurrentAppStatus:         "failed",
				CurrentECVersion:         "2.0.0+k8s-1.29",
				TargetAppVersion:         "1.5.0",
				TargetAppSequence:        500,
				TargetECVersion:          "2.1.0+k8s-1.29",
				License:                  newTestLicense(true),
				requiredReleases:         []string{},
				currentReleaseIsRequired: true,
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "current release failed and is deployed",
			opts: UpgradableOptions{
				CurrentAppVersion:        "1.0.0",
				CurrentAppSequence:       100,
				CurrentAppStatus:         "deployed",
				CurrentECVersion:         "2.0.0+k8s-1.29",
				TargetAppVersion:         "1.5.0",
				TargetAppSequence:        500,
				TargetECVersion:          "2.1.0+k8s-1.29",
				License:                  newTestLicense(true),
				requiredReleases:         []string{},
				currentReleaseIsRequired: true,
			},
			expectError:         false,
			expectValidationErr: false,
		},
		{
			name: "ec version downgrade",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.5.0+k8s-1.30",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.3.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "k8s version skip - one minor version",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.31",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "k8s version skip - multiple minor versions",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.27",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.31",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "k8s version downgrade",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.33",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.32",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: true,
		},
		{
			name: "invalid semver in app version",
			opts: UpgradableOptions{
				CurrentAppVersion:  "invalid-version",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+k8s-1.29",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: false, // parsing errors are not ValidationError
		},
		{
			name: "invalid ec version format",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "invalid-version",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: false, // parsing errors are not ValidationError
		},
		{
			name: "invalid k8s version format in ec version",
			opts: UpgradableOptions{
				CurrentAppVersion:  "1.0.0",
				CurrentAppSequence: 100,
				CurrentECVersion:   "2.0.0+invalid-build",
				TargetAppVersion:   "1.1.0",
				TargetAppSequence:  101,
				TargetECVersion:    "2.1.0+k8s-1.29",
				License:            newTestLicense(true),
				requiredReleases:   []string{},
			},
			expectError:         true,
			expectValidationErr: false, // parsing errors are not ValidationError
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIsReleaseUpgradable(t.Context(), tt.opts)

			if tt.expectError {
				require.Error(t, err)

				// Check if it's a ValidationError type
				var validationErr *ValidationError
				isValidationErr := errors.As(err, &validationErr)

				if tt.expectValidationErr {
					assert.True(t, isValidationErr, "expected ValidationError but got: %T", err)
				} else {
					assert.False(t, isValidationErr, "expected non-ValidationError but got ValidationError")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
