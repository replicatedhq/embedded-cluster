package runtimeconfig

import (
	"os"
	"path/filepath"

	"github.com/gosimple/slug"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
)

// DefaultNoProxy holds the default no proxy values.
var DefaultNoProxy = []string{
	// localhost
	"localhost", "127.0.0.1",
	// kubernetes
	".cluster.local", ".svc",
	// cloud metadata service
	"169.254.169.254",
}

const DefaultReplicatedAppDomain = "replicated.app"
const DefaultProxyRegistryDomain = "proxy.replicated.com"
const DefaultReplicatedRegistryDomain = "registry.replicated.com"
const KotsadmNamespace = "kotsadm"
const KotsadmServiceAccount = "kotsadm"
const SeaweedFSNamespace = "seaweedfs"
const RegistryNamespace = "registry"
const VeleroNamespace = "velero"
const EmbeddedClusterNamespace = "embedded-cluster"

// BinaryName returns the binary name, this is useful for places where we
// need to present the name of the binary to the user (the name may vary if
// the binary is renamed). We make sure the name does not contain invalid
// characters for a filename.
func BinaryName() string {
	exe, err := os.Executable()
	if err != nil {
		logrus.Fatalf("unable to get executable path: %s", err)
	}
	base := filepath.Base(exe)
	return slug.Make(base)
}

// EmbeddedClusterLogsSubDir returns the path to the directory where embedded-cluster logs
// are stored.
func EmbeddedClusterLogsSubDir() string {
	path := "/var/log/embedded-cluster"
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster logs dir: %s", err)
	}
	return path
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func PathToLog(name string) string {
	return filepath.Join(EmbeddedClusterLogsSubDir(), name)
}

// K0sBinaryPath returns the path to the k0s binary when it is installed on the node. This
// does not return the binary just after we materialized it but the path we want it to be
// once it is installed.
func K0sBinaryPath() string {
	return "/usr/local/bin/k0s"
}

// PathToK0sStatusSocket returns the full path to the k0s status socket.
func PathToK0sStatusSocket() string {
	return "/run/k0s/status.sock"
}

// PathToK0sConfig returns the full path to the k0s configuration file.
func PathToK0sConfig() string {
	return "/etc/k0s/k0s.yaml"
}

// PathToK0sContainerdConfig returns the full path to the k0s containerd configuration directory
func PathToK0sContainerdConfig() string {
	return "/etc/k0s/containerd.d/"
}

// PathToECConfig returns the full path to the embedded cluster configuration file.
// This file is used to specify the embedded cluster data directory.
func PathToECConfig() string {
	return "/etc/embedded-cluster/ec.yaml"
}

// GetDomains returns the domains for the embedded cluster. The first priority is the domains configured within the provided config spec.
// The second priority is the domains configured within the channel release. If neither is configured, the default domains are returned.
func GetDomains(cfgspec *ecv1beta1.ConfigSpec) ecv1beta1.Domains {
	replicatedAppDomain := DefaultReplicatedAppDomain
	proxyRegistryDomain := DefaultProxyRegistryDomain
	replicatedRegistryDomain := DefaultReplicatedRegistryDomain

	// get defaults from channel release if available
	rel := release.GetChannelRelease()
	if rel != nil {
		if rel.DefaultDomains.ReplicatedAppDomain != "" {
			replicatedAppDomain = rel.DefaultDomains.ReplicatedAppDomain
		}
		if rel.DefaultDomains.ProxyRegistryDomain != "" {
			proxyRegistryDomain = rel.DefaultDomains.ProxyRegistryDomain
		}
		if rel.DefaultDomains.ReplicatedRegistryDomain != "" {
			replicatedRegistryDomain = rel.DefaultDomains.ReplicatedRegistryDomain
		}
	}

	// get overrides from config spec if available
	if cfgspec != nil {
		if cfgspec.Domains.ReplicatedAppDomain != "" {
			replicatedAppDomain = cfgspec.Domains.ReplicatedAppDomain
		}
		if cfgspec.Domains.ProxyRegistryDomain != "" {
			proxyRegistryDomain = cfgspec.Domains.ProxyRegistryDomain
		}
		if cfgspec.Domains.ReplicatedRegistryDomain != "" {
			replicatedRegistryDomain = cfgspec.Domains.ReplicatedRegistryDomain
		}
	}

	return ecv1beta1.Domains{
		ReplicatedAppDomain:      replicatedAppDomain,
		ProxyRegistryDomain:      proxyRegistryDomain,
		ReplicatedRegistryDomain: replicatedRegistryDomain,
	}
}
