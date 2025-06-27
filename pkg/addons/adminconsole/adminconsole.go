package adminconsole

import (
	"fmt"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
)

const (
	_releaseName = "admin-console"

	_namespace = constants.KotsadmNamespace
)

var _ types.AddOn = (*AdminConsole)(nil)

type AdminConsole struct {
	IsAirgap           bool
	IsHA               bool
	Proxy              *ecv1beta1.ProxySpec
	ServiceCIDR        string
	IsMultiNodeEnabled bool
	HostCABundlePath   string
	DataDir            string
	K0sDataDir         string
	AdminConsolePort   int

	// These options are only used during installation
	Password      string
	TLSCertBytes  []byte
	TLSKeyBytes   []byte
	Hostname      string
	KotsInstaller KotsInstaller

	// DryRun is a flag to enable dry-run mode for Admin Console.
	// If true, Admin Console will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

type KotsInstaller func() error

func (a *AdminConsole) Name() string {
	return "Admin Console"
}

func (a *AdminConsole) Version() string {
	return Metadata.Version
}

func (a *AdminConsole) ReleaseName() string {
	return _releaseName
}

func (a *AdminConsole) Namespace() string {
	return _namespace
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	}
}

func (a *AdminConsole) ChartLocation(domains ecv1beta1.Domains) string {
	chartName := Metadata.Location
	if AdminConsoleChartRepoOverride != "" {
		chartName = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", AdminConsoleChartRepoOverride)
	}

	if domains.ProxyRegistryDomain != "" {
		chartName = strings.Replace(chartName, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
	}
	return chartName
}

// DryRunManifests returns the manifests generated during a dry run.
func (a *AdminConsole) DryRunManifests() [][]byte {
	return a.dryRunManifests
}
