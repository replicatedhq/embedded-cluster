package appupgrademanager

import (
	"context"

	appupgradestore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ AppUpgradeManager = &appUpgradeManager{}

// AppUpgradeManager provides methods for managing app upgrades
type AppUpgradeManager interface {
	// Upgrade upgrades the app with the provided config values
	Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error
}

// appUpgradeManager is an implementation of the AppUpgradeManager interface
type appUpgradeManager struct {
	appUpgradeStore       appupgradestore.Store
	releaseData           *release.ReleaseData
	license               []byte
	clusterID             string
	airgapBundle          string
	kotsCLI               kotscli.KotsCLI
	kcli                  client.Client
	kubernetesEnvSettings *helmcli.EnvSettings
	logger                logrus.FieldLogger
}

type AppUpgradeManagerOption func(*appUpgradeManager)

func WithLogger(logger logrus.FieldLogger) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.logger = logger
	}
}

func WithAppUpgradeStore(store appupgradestore.Store) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.appUpgradeStore = store
	}
}

func WithReleaseData(releaseData *release.ReleaseData) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.releaseData = releaseData
	}
}

func WithClusterID(clusterID string) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.clusterID = clusterID
	}
}

func WithAirgapBundle(airgapBundle string) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.airgapBundle = airgapBundle
	}
}

func WithLicense(license []byte) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.license = license
	}
}

func WithKotsCLI(kotsCLI kotscli.KotsCLI) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.kotsCLI = kotsCLI
	}
}

func WithKubeClient(kcli client.Client) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.kcli = kcli
	}
}

func WithKubernetesEnvSettings(envSettings *helmcli.EnvSettings) AppUpgradeManagerOption {
	return func(m *appUpgradeManager) {
		m.kubernetesEnvSettings = envSettings
	}
}

// NewAppUpgradeManager creates a new AppUpgradeManager with the provided options
func NewAppUpgradeManager(opts ...AppUpgradeManagerOption) (*appUpgradeManager, error) {
	manager := &appUpgradeManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.appUpgradeStore == nil {
		manager.appUpgradeStore = appupgradestore.NewMemoryStore()
	}

	return manager, nil
}
