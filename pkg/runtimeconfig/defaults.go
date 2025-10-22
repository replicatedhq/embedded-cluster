package runtimeconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gosimple/slug"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	K0sBinaryPath           = "/usr/local/bin/k0s"
	K0sStatusSocketPath     = "/run/k0s/status.sock"
	K0sConfigPath           = "/etc/k0s/k0s.yaml"
	K0sContainerdConfigPath = "/etc/k0s/containerd.d/"
	ECConfigPath            = "/etc/embedded-cluster/ec.yaml"
)

// AppSlug returns the intended binary name. This is the app slug when a release is embedded,
// otherwise it is the basename of the executable. This is useful for places where we need to
// present the name of the binary to the user. We make sure the name does not contain invalid
// characters for a filename.
func AppSlug() string {
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

// KotsadmNamespace returns the namespace where the kots app and admin console should be deployed.
// If "kotsadm" exists, it returns "kotsadm" for backwards compatibility, otherwise it returns the app slug.
func KotsadmNamespace(ctx context.Context, kcli client.Client) (string, error) {
	// Install scenario - no cluster exists yet, use app slug
	if kcli == nil {
		return AppSlug(), nil
	}

	// Upgrade scenario - check if kotsadm namespace exists and use it for backwards compatibility
	err := kcli.Get(ctx, client.ObjectKey{Name: constants.KotsadmNamespace}, &corev1.Namespace{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return AppSlug(), nil
		}
		return "", fmt.Errorf("failed to get namespace %s: %w", constants.KotsadmNamespace, err)
	}

	return constants.KotsadmNamespace, nil
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
