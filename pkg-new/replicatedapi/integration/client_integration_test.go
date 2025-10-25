package integration

import (
	"context"
	_ "embed"
	"log"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "sigs.k8s.io/yaml"
)

//go:embed testdata/license.yaml
var testLicenseYAML []byte

var testLicense kotsv1beta1.License

func init() {
	err := kyaml.Unmarshal(testLicenseYAML, &testLicense)
	if err != nil {
		log.Panicf("Failed to unmarshal license YAML: %v", err)
	}
}

// TestSyncLicense tests syncing a license with a specific channel ID
func TestSyncLicense(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	req := require.New(t)

	// Use the channel ID from the license
	channelID := testLicense.Spec.ChannelID
	if channelID == "" {
		t.Skip("License does not have a channel ID")
	}

	t.Logf("Testing with channel ID: %s", channelID)

	// Create release data with channel information
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			ChannelID: channelID,
		},
	}

	// Create client with real endpoint
	client, err := replicatedapi.NewClient(testLicense.Spec.Endpoint, &testLicense, releaseData)
	req.NoError(err, "Failed to create client")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sync license
	reportingInfo := &replicatedapi.ReportingInfo{
		EmbeddedClusterNodes: stringPtr("3"),
		AppStatus:            stringPtr("ready"),
	}
	latestLicense, rawLicense, err := client.SyncLicense(ctx, reportingInfo)

	req.NoError(err, "Failed to sync license with channel ID")
	req.NotNil(latestLicense)
	req.NotNil(rawLicense)

	t.Logf("Successfully synced license for channel %s:", channelID)
	t.Logf("  Sequence: %d", latestLicense.Spec.LicenseSequence)
	t.Logf("  Channel Name: %s", latestLicense.Spec.ChannelName)

	// Validate the channel matches
	assert.Equal(t, channelID, latestLicense.Spec.ChannelID, "Channel ID should match requested channel")
}

func stringPtr(s string) *string {
	return &s
}
