package runtimeconfig

import (
	"os"
	"path/filepath"

	"github.com/gosimple/slug"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
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

const (
	KotsadmNamespace         = "kotsadm"
	KotsadmServiceAccount    = "kotsadm"
	SeaweedFSNamespace       = "seaweedfs"
	RegistryNamespace        = "registry"
	VeleroNamespace          = "velero"
	EmbeddedClusterNamespace = "embedded-cluster"
)

const (
	K0sBinaryPath           = "/usr/local/bin/k0s"
	K0sStatusSocketPath     = "/run/k0s/status.sock"
	K0sConfigPath           = "/etc/k0s/k0s.yaml"
	K0sContainerdConfigPath = "/etc/k0s/containerd.d/"
	ECConfigPath            = "/etc/embedded-cluster/ec.yaml"
)

// BinaryName returns the intended binary name. This is the app slug when a release is embedded,
// otherwise it is the basename of the executable. This is useful for places where we need to
// present the name of the binary to the user. We make sure the name does not contain invalid
// characters for a filename.
func BinaryName() string {
	var name string
	if release := release.GetChannelRelease(); release != nil {
		name = release.AppSlug
	} else {
		exe, err := os.Executable()
		if err != nil {
			logrus.Fatalf("unable to get executable path: %s", err)
		}
		name = filepath.Base(exe)
	}
	return slug.Make(name)
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

// GetDomains returns the domains for the embedded cluster. The first priority is the domains configured within the provided config spec.
// The second priority is the domains configured within the channel release. If neither is configured, the default domains are returned.
func GetDomains(cfgspec *ecv1beta1.ConfigSpec) ecv1beta1.Domains {
	return domains.GetDomains(cfgspec, release.GetChannelRelease())
}
