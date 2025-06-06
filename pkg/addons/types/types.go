package types

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

type LogFunc func(format string, args ...interface{})

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, opts InstallOptions, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, writer *spinner.MessageWriter, opts InstallOptions, overrides []string) error
	// TODO: message writer for enable HA
	Upgrade(ctx context.Context, opts InstallOptions, overrides []string) error
}

type InstallOptions struct {
	ClusterID                 string
	AdminConsolePassword      string
	IsAirgap                  bool
	IsHA                      bool
	Proxy                     *ecv1beta1.ProxySpec
	TLSCertBytes              []byte
	TLSKeyBytes               []byte
	Hostname                  string
	ServiceCIDR               string
	IsDisasterRecoveryEnabled bool
	IsMultiNodeEnabled        bool
	EmbeddedConfigSpec        *ecv1beta1.ConfigSpec
	EndUserConfigSpec         *ecv1beta1.ConfigSpec
	Domains                   ecv1beta1.Domains
	KotsInstaller             KotsInstaller

	IsRestore bool

	// IsDryRun is a flag to enable dry-run mode.
	// If true, Install and Upgrade will only render the helm template and additional manifests,
	// but not install the release.
	IsDryRun bool
}

type KotsInstaller func(msg *spinner.MessageWriter) error
