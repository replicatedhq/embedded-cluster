package helpers

import (
	"os"
	"runtime"
)

var (
	_clusterArch = runtime.GOARCH
	_clusterOS   = "linux"
)

func init() {
	if val := os.Getenv("CLUSTER_ARCH"); val != "" {
		SetClusterArch(val)
	}
}

// ClusterArch returns the architecture of the cluster. This defaults to the architecture the
// binary is compiled for (this should be the CPU architecture of the host). This can be overridden
// by setting the CLUSTER_ARCH environment variable or set via SetClusterArch.
func ClusterArch() string {
	return _clusterArch
}

// SetClusterArch sets the architecture of the cluster.
func SetClusterArch(arch string) {
	_clusterArch = arch
}

// ClusterOS returns the operating system of the cluster. This is hardcoded to "linux".
func ClusterOS() string {
	return _clusterOS
}
