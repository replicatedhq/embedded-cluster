package adminconsole

import (
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
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
	logf types.LogFunc

	dryRunManifests [][]byte
}

type Option func(*AdminConsole)

func New(opts ...Option) *AdminConsole {
	addon := &AdminConsole{}
	for _, opt := range opts {
		opt(addon)
	}
	if addon.logf == nil {
		addon.logf = slog.Info
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *AdminConsole) {
		a.logf = logf
	}
}

func init() {
	if AdminConsoleImageOverride != "" {
		helmValues["images"].(map[string]any)["kotsadm"] = AdminConsoleImageOverride
	}
	if AdminConsoleMigrationsImageOverride != "" {
		helmValues["images"].(map[string]any)["migrations"] = AdminConsoleMigrationsImageOverride
	}
	if AdminConsoleKurlProxyImageOverride != "" {
		helmValues["images"].(map[string]any)["kurlProxy"] = AdminConsoleKurlProxyImageOverride
	}
}

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

// DryRunManifests returns the manifests generated during a dry run.
func (a *AdminConsole) DryRunManifests() [][]byte {
	return a.dryRunManifests
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

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	}
}
