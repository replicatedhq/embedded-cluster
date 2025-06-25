package addons

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOnsInterface interface {
	// Install installs all addons
	Install(ctx context.Context, opts InstallOptions) error
	// Upgrade upgrades all addons
	Upgrade(ctx context.Context, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata, opts UpgradeOptions) error
	// CanEnableHA checks if high availability can be enabled in the cluster
	CanEnableHA(context.Context) (bool, string, error)
	// EnableHA enables high availability for the cluster
	EnableHA(ctx context.Context, opts EnableHAOptions, spinner *spinner.MessageWriter) error
	// EnableAdminConsoleHA enables high availability for the admin console
	EnableAdminConsoleHA(ctx context.Context, opts EnableHAOptions) error
}

var _ AddOnsInterface = (*AddOns)(nil)

type AddOns struct {
	logf     types.LogFunc
	hcli     helm.Client
	kcli     client.Client
	mcli     metadata.Interface
	kclient  kubernetes.Interface
	domains  ecv1beta1.Domains
	progress chan<- types.AddOnProgress
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

func WithDomains(domains ecv1beta1.Domains) AddOnsOption {
	return func(a *AddOns) {
		a.domains = domains
	}
}

func WithProgressChannel(progress chan types.AddOnProgress) AddOnsOption {
	return func(a *AddOns) {
		a.progress = progress
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

	return a
}
