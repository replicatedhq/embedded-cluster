package install

import (
	"context"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ AppInstallManager = &appInstallManager{}

// AppInstallManager provides methods for managing app installation
type AppInstallManager interface {
	// Install installs the app with the provided Helm charts
	Install(ctx context.Context, installableCharts []types.InstallableHelmChart, configValues types.AppConfigValues, registrySettings *types.RegistrySettings, hostCABundlePath string) error
}

// appInstallManager is an implementation of the AppInstallManager interface
type appInstallManager struct {
	appInstallStore       appinstallstore.Store
	releaseData           *release.ReleaseData
	license               []byte
	clusterID             string
	airgapBundle          string
	hcli                  helm.Client
	kcli                  client.Client
	mcli                  metadata.Interface
	kubernetesEnvSettings *helmcli.EnvSettings
	logger                logrus.FieldLogger
}

type AppInstallManagerOption func(*appInstallManager)

func WithLogger(logger logrus.FieldLogger) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.logger = logger
	}
}

func WithAppInstallStore(store appinstallstore.Store) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.appInstallStore = store
	}
}

func WithReleaseData(releaseData *release.ReleaseData) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.releaseData = releaseData
	}
}

func WithLicense(license []byte) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.license = license
	}
}

func WithClusterID(clusterID string) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.clusterID = clusterID
	}
}

func WithAirgapBundle(airgapBundle string) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.airgapBundle = airgapBundle
	}
}

func WithHelmClient(hcli helm.Client) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.hcli = hcli
	}
}

func WithKubeClient(kcli client.Client) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.mcli = mcli
	}
}

func WithKubernetesEnvSettings(envSettings *helmcli.EnvSettings) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.kubernetesEnvSettings = envSettings
	}
}

// NewAppInstallManager creates a new AppInstallManager with the provided options
func NewAppInstallManager(opts ...AppInstallManagerOption) (*appInstallManager, error) {
	manager := &appInstallManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.appInstallStore == nil {
		manager.appInstallStore = appinstallstore.NewMemoryStore()
	}

	return manager, nil
}
