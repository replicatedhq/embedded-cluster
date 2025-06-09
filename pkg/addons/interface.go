package addons

import (
	"context"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOnsInterface interface {
	// Versions returns the versions of all addons
	Versions() map[string]string
	// GenerateChartConfigs generates the chart configurations for all addons
	GenerateChartConfigs() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error)
	// GetImages returns all images required by the addons
	GetImages() []string
	// GetAdditionalImages returns additional images required by the addons
	GetAdditionalImages() []string
	// Install installs all addons
	Install(ctx context.Context, logf types.LogFunc, hcli helm.Client, rc runtimeconfig.RuntimeConfig, opts InstallOptions) error
	// Upgrade upgrades all addons
	Upgrade(ctx context.Context, logf types.LogFunc, hcli helm.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error
	// CanEnableHA checks if high availability can be enabled in the cluster
	CanEnableHA(ctx context.Context, kcli client.Client) (bool, string, error)
	// EnableHA enables high availability for the cluster
	EnableHA(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, kclient kubernetes.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, serviceCIDR string, inSpec ecv1beta1.InstallationSpec, spinner *spinner.MessageWriter) error
	// EnableAdminConsoleHA enables high availability for the admin console
	EnableAdminConsoleHA(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, isAirgap bool, serviceCIDR string, proxy *ecv1beta1.ProxySpec, cfgspec *ecv1beta1.ConfigSpec, licenseInfo *ecv1beta1.LicenseInfo)
}

type AddOns struct {
	logf    types.LogFunc
	hcli    helm.Client
	kcli    client.Client
	mcli    metadata.Interface
	kclient kubernetes.Interface
	rc      runtimeconfig.RuntimeConfig
}

type AddOnsOption func(*AddOns)

func WithLogFunc(logf types.LogFunc) AddOnsOption {
	return func(a *AddOns) {
		a.logf = logf
	}
}

func WithHelmClient(hcli helm.Client) AddOnsOption {
	return func(a *AddOns) {
		a.hcli = hcli
	}
}

func WithKubernetesClient(kcli client.Client) AddOnsOption {
	return func(a *AddOns) {
		a.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) AddOnsOption {
	return func(a *AddOns) {
		a.mcli = mcli
	}
}

func WithKubernetesClientSet(kclient kubernetes.Interface) AddOnsOption {
	return func(a *AddOns) {
		a.kclient = kclient
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) AddOnsOption {
	return func(a *AddOns) {
		a.rc = rc
	}
}

func New(opts ...AddOnsOption) *AddOns {
	a := &AddOns{}
	for _, opt := range opts {
		opt(a)
	}

	if a.logf == nil {
		a.logf = logrus.Debugf
	}

	if a.rc == nil {
		a.rc = runtimeconfig.New(nil)
	}

	return a
}
