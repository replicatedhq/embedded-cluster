package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/kinds/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// clusterNodesMatchVersion returns true if all nodes in the cluster have kubeletVersion matching the provided version.
func clusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error) {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return false, fmt.Errorf("list nodes: %w", err)
	}
	for _, node := range nodes.Items {
		if node.Status.NodeInfo.KubeletVersion != version {
			return false, nil
		}
	}
	return true, nil
}
