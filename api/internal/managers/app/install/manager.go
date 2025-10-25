package install

import (
	"context"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ AppInstallManager = &appInstallManager{}

// AppInstallManager provides methods for managing app installation
type AppInstallManager interface {
	// Install installs the app with the provided config values
	Install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error
	// GetStatus returns the current app installation status
	GetStatus() (types.AppInstall, error)
}

// appInstallManager is an implementation of the AppInstallManager interface
type appInstallManager struct {
	appInstallStore       appinstallstore.Store
	releaseData           *release.ReleaseData
	license               []byte
	clusterID             string
	airgapBundle          string
	kotsCLI               kotscli.KotsCLI
	kcli                  client.Client
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

func WithKotsCLI(kotsCLI kotscli.KotsCLI) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.kotsCLI = kotsCLI
	}
}

func WithKubeClient(kcli client.Client) AppInstallManagerOption {
	return func(m *appInstallManager) {
		m.kcli = kcli
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
