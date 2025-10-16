package upgrade

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metadata"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfraUpgrader provides methods for performing infrastructure upgrades
type InfraUpgrader interface {
	// CreateInstallation creates the installation object in the cluster
	CreateInstallation(ctx context.Context, in *ecv1beta1.Installation) error

	// CopyVersionMetadataToCluster copies version metadata to the cluster
	CopyVersionMetadataToCluster(ctx context.Context, in *ecv1beta1.Installation) error

	// DistributeArtifacts distributes artifacts to nodes and cluster
	DistributeArtifacts(ctx context.Context, in *ecv1beta1.Installation, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string) error

	// UpgradeK0s upgrades the k0s cluster to the version specified in the installation
	UpgradeK0s(ctx context.Context, in *ecv1beta1.Installation) error

	// UpdateClusterConfig updates the k0s cluster configuration
	UpdateClusterConfig(ctx context.Context, in *ecv1beta1.Installation) error

	// UpgradeAddons upgrades all addon charts
	UpgradeAddons(ctx context.Context, in *ecv1beta1.Installation, progressChan chan addontypes.AddOnProgress) error

	// UpgradeExtensions upgrades all extensions
	UpgradeExtensions(ctx context.Context, in *ecv1beta1.Installation) error

	// CreateHostSupportBundle creates a host support bundle after upgrade
	CreateHostSupportBundle(ctx context.Context) error
}

// infraUpgrader is an implementation of the InfraUpgrader interface
type infraUpgrader struct {
	kubeClient    client.Client
	helmClient    helm.Client
	runtimeConfig runtimeconfig.RuntimeConfig
	logger        logrus.FieldLogger
}

type InfraUpgraderOption func(*infraUpgrader)

func WithKubeClient(cli client.Client) InfraUpgraderOption {
	return func(u *infraUpgrader) {
		u.kubeClient = cli
	}
}

func WithHelmClient(hcli helm.Client) InfraUpgraderOption {
	return func(u *infraUpgrader) {
		u.helmClient = hcli
	}
}

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) InfraUpgraderOption {
	return func(u *infraUpgrader) {
		u.runtimeConfig = rc
	}
}

func WithLogger(logger logrus.FieldLogger) InfraUpgraderOption {
	return func(u *infraUpgrader) {
		u.logger = logger
	}
}

// NewInfraUpgrader creates a new InfraUpgrader with the provided options
func NewInfraUpgrader(opts ...InfraUpgraderOption) InfraUpgrader {
	u := &infraUpgrader{}

	for _, opt := range opts {
		opt(u)
	}

	if u.logger == nil {
		u.logger = logrus.New()
	}

	return u
}

// UpgradeK0s upgrades the k0s cluster to the version specified in the installation
func (u *infraUpgrader) UpgradeK0s(ctx context.Context, in *ecv1beta1.Installation) error {
	return upgradeK0s(ctx, u.kubeClient, u.runtimeConfig, in, u.logger)
}

// UpdateClusterConfig updates the k0s cluster configuration
func (u *infraUpgrader) UpdateClusterConfig(ctx context.Context, in *ecv1beta1.Installation) error {
	return updateClusterConfig(ctx, u.kubeClient, in, u.logger)
}

// UpgradeAddons upgrades all addon charts
func (u *infraUpgrader) UpgradeAddons(ctx context.Context, in *ecv1beta1.Installation, progressChan chan addontypes.AddOnProgress) error {
	return upgradeAddons(ctx, u.kubeClient, u.helmClient, u.runtimeConfig, in, progressChan, u.logger)
}

// UpgradeExtensions upgrades all extensions
func (u *infraUpgrader) UpgradeExtensions(ctx context.Context, in *ecv1beta1.Installation) error {
	return upgradeExtensions(ctx, u.kubeClient, u.helmClient, in, u.logger)
}

// CreateHostSupportBundle creates a host support bundle after upgrade
func (u *infraUpgrader) CreateHostSupportBundle(ctx context.Context) error {
	return support.CreateHostSupportBundle(ctx, u.kubeClient)
}

// CreateInstallation creates the installation object in the cluster
func (u *infraUpgrader) CreateInstallation(ctx context.Context, in *ecv1beta1.Installation) error {
	return CreateInstallation(ctx, u.kubeClient, in, u.logger)
}

// CopyVersionMetadataToCluster copies version metadata to the cluster
func (u *infraUpgrader) CopyVersionMetadataToCluster(ctx context.Context, in *ecv1beta1.Installation) error {
	return metadata.CopyVersionMetadataToCluster(ctx, u.kubeClient, in)
}

// DistributeArtifacts distributes artifacts to nodes and cluster
func (u *infraUpgrader) DistributeArtifacts(ctx context.Context, in *ecv1beta1.Installation, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string) error {
	return distributeArtifacts(ctx, u.kubeClient, u.runtimeConfig, in, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion)
}
