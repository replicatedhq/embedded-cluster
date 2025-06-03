package runtimeconfig

import (
	"os"
	"path/filepath"

	"github.com/gosimple/slug"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg-new/paths"
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
	path := paths.EmbeddedClusterLogsSubDir()
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster logs dir: %s", err)
	}
	return path
}

// GetDomains returns the domains for the embedded cluster. The first priority is the domains configured within the provided config spec.
// The second priority is the domains configured within the channel release. If neither is configured, the default domains are returned.
func GetDomains(cfgspec *ecv1beta1.ConfigSpec) ecv1beta1.Domains {
	return domains.GetDomains(cfgspec, release.GetChannelRelease())
}
