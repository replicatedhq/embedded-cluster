package adminconsole

import (
	"fmt"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "admin-console"
	namespace   = runtimeconfig.KotsadmNamespace
)

var (
	// Overwritten by -ldflags in Makefile
	AdminConsoleChartRepoOverride       = ""
	AdminConsoleImageOverride           = ""
	AdminConsoleMigrationsImageOverride = ""
	AdminConsoleKurlProxyImageOverride  = ""
	KotsVersion                         = ""
)

var _ types.AddOn = (*AdminConsole)(nil)

type AdminConsole struct {
	IsAirgap                 bool
	IsHA                     bool
	Proxy                    *ecv1beta1.ProxySpec
	ServiceCIDR              string
	Password                 string
	TLSCertBytes             []byte
	TLSKeyBytes              []byte
	Hostname                 string
	KotsInstaller            KotsInstaller
	IsMultiNodeEnabled       bool
	ReplicatedAppDomain      string
	ProxyRegistryDomain      string
	ReplicatedRegistryDomain string
	HostCABundlePath         string

	// DryRun is a flag to enable dry-run mode for Admin Console.
	// If true, Admin Console will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

type KotsInstaller func(msg *spinner.MessageWriter) error

func (a *AdminConsole) Name() string {
	return "Admin Console"
}

func (a *AdminConsole) Version() string {
	return Metadata.Version
}

func (a *AdminConsole) ReleaseName() string {
	return releaseName
}

func (a *AdminConsole) Namespace() string {
	return namespace
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	}
}

func (a *AdminConsole) ChartLocation() string {
	chartName := Metadata.Location
	if AdminConsoleChartRepoOverride != "" {
		chartName = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", AdminConsoleChartRepoOverride)
	}

	if a.ProxyRegistryDomain != "" {
		chartName = strings.Replace(chartName, "proxy.replicated.com", a.ProxyRegistryDomain, 1)
	}
	return chartName
}

// DryRunManifests returns the manifests generated during a dry run.
func (a *AdminConsole) DryRunManifests() [][]byte {
	return a.dryRunManifests
}
