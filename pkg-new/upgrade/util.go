package upgrade

import (
	"strings"

	"github.com/replicatedhq/embedded-cluster/kinds/types"
)

// k0sVersionFromMetadata takes versions like v1.30.5+k0s.0 and returns v1.30.5+k0s to match the kubeletVersion in a cluster
func k0sVersionFromMetadata(meta *types.ReleaseMetadata) string {
	if meta == nil || meta.Versions == nil {
		return ""
	}
	if _, ok := meta.Versions["Kubernetes"]; !ok {
		return ""
	}
	desiredParts := strings.Split(meta.Versions["Kubernetes"], "k0s")
	desiredVersion := desiredParts[0] + "k0s"
	return desiredVersion
}
