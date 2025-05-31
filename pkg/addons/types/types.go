package types

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type LogFunc func(format string, args ...interface{})

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, opts InstallOptions, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, writer *spinner.MessageWriter, opts InstallOptions, overrides []string) error
	Upgrade(ctx context.Context, writer *spinner.MessageWriter, opts InstallOptions, overrides []string) error
}

type InstallOptions struct {
	AdminConsolePassword     string
	AdminConsolePort         int
	License                  *kotsv1beta1.License
	IsAirgap                 bool
	Proxy                    *ecv1beta1.ProxySpec
	HostCABundlePath         string
	TLSCertBytes             []byte
	TLSKeyBytes              []byte
	Hostname                 string
	ServiceCIDR              string
	DisasterRecoveryEnabled  bool
	IsMultiNodeEnabled       bool
	EmbeddedConfigSpec       *ecv1beta1.ConfigSpec
	EndUserConfigSpec        *ecv1beta1.ConfigSpec
	Domains                  ecv1beta1.Domains
	KotsInstaller            KotsInstaller
	ClusterID                string
	EmbeddedClusterHomeDir   string
	EmbeddedClusterK0sSubDir string
	IsHA                     bool
	IsRestore                bool

	// IsDryRun is a flag to enable dry-run mode.
	// If true, Install and Upgrade will only render the helm template and additional manifests,
	// but not install the release.
	IsDryRun bool
}

type KotsInstaller func(msg *spinner.MessageWriter) error
