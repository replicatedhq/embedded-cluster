package main

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
)

func TestNewCluster(t *testing.T) {
	_ = cmx.NewCluster(context.Background(), cmx.ClusterInput{
		T:            t,
		Nodes:        5,
		Distribution: "ubuntu",
		Version:      "22.04",
		WithProxy:    true,
		// AirgapInstallBundlePath: "/tmp/airgap-install-bundle.tar.gz",
		// AirgapUpgradeBundlePath: "/tmp/airgap-upgrade-bundle.tar.gz",
	})
	// defer cluster.Cleanup()
}
