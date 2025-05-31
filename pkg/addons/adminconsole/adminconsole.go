package adminconsole

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseName = "admin-console"
	namespace   = runtimeconfig.KotsadmNamespace
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
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
	kcli client.Client
	mcli metadata.Interface
	hcli helm.Client

	dryRunManifests [][]byte
}

type Option func(*AdminConsole)

func New(opts ...Option) *AdminConsole {
	addon := &AdminConsole{}
	for _, opt := range opts {
		opt(addon)
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *AdminConsole) {
		a.logf = logf
	}
}

func WithClients(kcli client.Client, mcli metadata.Interface, hcli helm.Client) Option {
	return func(a *AdminConsole) {
		a.kcli = kcli
		a.mcli = mcli
		a.hcli = hcli
	}
}

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(errors.Wrap(err, "unmarshal metadata"))
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unmarshal values"))
	}
	helmValues = hv

	helmValues["embeddedClusterVersion"] = versions.Version

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

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	}
}

func ChartLocation(domains ecv1beta1.Domains) string {
	chartName := Metadata.Location
	if AdminConsoleChartRepoOverride != "" {
		chartName = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", AdminConsoleChartRepoOverride)
	}

	if domains.ProxyRegistryDomain != "" {
		chartName = strings.Replace(chartName, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
	}
	return chartName
}
