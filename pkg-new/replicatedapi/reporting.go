package replicatedapi

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

const (
	DistributionEmbeddedCluster string = "embedded-cluster"
)

func (c *client) injectReportingInfoHeaders(header http.Header) {
	for key, value := range c.getReportingInfoHeaders() {
		header.Set(key, value)
	}
}

func (c *client) getReportingInfoHeaders() map[string]string {
	headers := make(map[string]string)

	// add headers from client
	channel, _ := c.getChannelFromLicense() // ignore error
	if channel != nil {
		headers["X-Replicated-DownstreamChannelID"] = channel.ChannelID
		headers["X-Replicated-DownstreamChannelName"] = channel.ChannelName
	}

	headers["X-Replicated-K8sVersion"] = versions.K0sVersion
	headers["X-Replicated-K8sDistribution"] = DistributionEmbeddedCluster
	headers["X-Replicated-EmbeddedClusterVersion"] = versions.Version

	// TODO
	// headers["X-Replicated-ClusterID"] = "TODO"
	// headers["X-Replicated-InstanceID"] = "TODO"
	headers["X-Replicated-EmbeddedClusterID"] = c.clusterID

	// Add static headers
	headers["X-Replicated-IsKurl"] = "false"

	// remove empty headers
	for key, value := range headers {
		if value == "" {
			delete(headers, key)
		}
	}

	return headers
}

// TODO: the following headers are injected by KOTS and are not yet supported by Embedded Cluster
// X-Replicated-EmbeddedClusterNodes
// X-Replicated-ReplHelmInstalls
// X-Replicated-NativeHelmInstalls
// X-Replicated-AppStatus
// X-Replicated-InstallStatus
// X-Replicated-PreflightStatus
// X-Replicated-DownstreamChannelSequence
// X-Replicated-DownstreamSequence
// X-Replicated-DownstreamSource
// X-Replicated-SkipPreflights
// X-Replicated-KotsInstallID
// X-Replicated-KurlInstallID
// X-Replicated-KurlNodeCountTotal
// X-Replicated-KurlNodeCountReady
// X-Replicated-IsGitOpsEnabled
// X-Replicated-GitOpsProvider
// X-Replicated-SnapshotProvider
// X-Replicated-SnapshotFullSchedule
// X-Replicated-SnapshotFullTTL
// X-Replicated-SnapshotPartialSchedule
// X-Replicated-SnapshotPartialTTL
