package utils

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

// GetDomains returns the configured custom domains for the release.
func GetDomains(releaseData *release.ReleaseData) ecv1beta1.Domains {
	var cfgspec *ecv1beta1.ConfigSpec
	if releaseData != nil && releaseData.EmbeddedClusterConfig != nil {
		cfgspec = &releaseData.EmbeddedClusterConfig.Spec
	}
	var rel *release.ChannelRelease
	if releaseData != nil && releaseData.ChannelRelease != nil {
		rel = releaseData.ChannelRelease
	}
	return domains.GetDomains(cfgspec, rel)
}
