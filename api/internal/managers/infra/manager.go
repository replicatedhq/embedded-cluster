package infra

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ InfraManager = &infraManager{}

// InfraManager provides methods for managing infrastructure setup
type InfraManager interface {
	Get() (*types.Infra, error)
	Install(ctx context.Context, config *types.InstallationConfig) error
}

// infraManager is an implementation of the InfraManager interface
type infraManager struct {
	infra         *types.Infra
	infraStore    Store
	rc            runtimeconfig.RuntimeConfig
	password      string
	tlsConfig     types.TLSConfig
	licenseFile   string
	airgapBundle  string
	configValues  string
	releaseData   *release.ReleaseData
	endUserConfig *ecv1beta1.Config
	logger        logrus.FieldLogger
	k0s           k0s.K0sInterface
	kcli          client.Client
	mcli          metadata.Interface
	hcli          helm.Client
	hostUtils     hostutils.HostUtilsInterface
	kotsInstaller func() error
	mu            sync.RWMutex
}

type InfraManagerOption func(*infraManager)

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) InfraManagerOption {
	return func(c *infraManager) {
		c.rc = rc
	}
}

func WithLogger(logger logrus.FieldLogger) InfraManagerOption {
	return func(c *infraManager) {
		c.logger = logger
	}
}

func WithInfra(infra *types.Infra) InfraManagerOption {
	return func(c *infraManager) {
		c.infra = infra
	}
}

func WithInfraStore(store Store) InfraManagerOption {
	return func(c *infraManager) {
		c.infraStore = store
	}
}

func WithPassword(password string) InfraManagerOption {
	return func(c *infraManager) {
		c.password = password
	}
}

func WithTLSConfig(tlsConfig types.TLSConfig) InfraManagerOption {
	return func(c *infraManager) {
		c.tlsConfig = tlsConfig
	}
}

func WithLicenseFile(licenseFile string) InfraManagerOption {
	return func(c *infraManager) {
		c.licenseFile = licenseFile
	}
}

func WithAirgapBundle(airgapBundle string) InfraManagerOption {
	return func(c *infraManager) {
		c.airgapBundle = airgapBundle
	}
}

func WithConfigValues(configValues string) InfraManagerOption {
	return func(c *infraManager) {
		c.configValues = configValues
	}
}

func WithReleaseData(releaseData *release.ReleaseData) InfraManagerOption {
	return func(c *infraManager) {
		c.releaseData = releaseData
	}
}

func WithEndUserConfig(endUserConfig *ecv1beta1.Config) InfraManagerOption {
	return func(c *infraManager) {
		c.endUserConfig = endUserConfig
	}
}

func WithK0s(k0s k0s.K0sInterface) InfraManagerOption {
	return func(c *infraManager) {
		c.k0s = k0s
	}
}

func WithKubeClient(kcli client.Client) InfraManagerOption {
	return func(c *infraManager) {
		c.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) InfraManagerOption {
	return func(c *infraManager) {
		c.mcli = mcli
	}
}

func WithHelmClient(hcli helm.Client) InfraManagerOption {
	return func(c *infraManager) {
		c.hcli = hcli
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) InfraManagerOption {
	return func(c *infraManager) {
		c.hostUtils = hostUtils
	}
}

func WithKotsInstaller(kotsInstaller func() error) InfraManagerOption {
	return func(c *infraManager) {
		c.kotsInstaller = kotsInstaller
	}
}

// NewInfraManager creates a new InfraManager with the provided options
func NewInfraManager(opts ...InfraManagerOption) *infraManager {
	manager := &infraManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.rc == nil {
		manager.rc = runtimeconfig.New(nil)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.infra == nil {
		manager.infra = &types.Infra{}
	}

	if manager.infraStore == nil {
		manager.infraStore = NewMemoryStore(manager.infra)
	}

	if manager.k0s == nil {
		manager.k0s = k0s.New()
	}

	if manager.hostUtils == nil {
		manager.hostUtils = hostutils.New()
	}

	return manager
}

func (m *infraManager) Get() (*types.Infra, error) {
	return m.infraStore.Get()
}
