package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	infrastore "github.com/replicatedhq/embedded-cluster/api/internal/store/infra"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ InfraManager = &infraManager{}

// InfraManager provides methods for managing infrastructure setup
type InfraManager interface {
	Get() (types.Infra, error)
	Install(ctx context.Context, ki kubernetesinstallation.Installation) error
}

// infraManager is an implementation of the InfraManager interface
type infraManager struct {
	infraStore            infrastore.Store
	password              string
	tlsConfig             types.TLSConfig
	license               []byte
	airgapBundle          string
	releaseData           *release.ReleaseData
	endUserConfig         *ecv1beta1.Config
	logger                logrus.FieldLogger
	kcli                  client.Client
	mcli                  metadata.Interface
	hcli                  helm.Client
	kubernetesEnvSettings *helmcli.EnvSettings
	mu                    sync.RWMutex
}

type InfraManagerOption func(*infraManager)

func WithLogger(logger logrus.FieldLogger) InfraManagerOption {
	return func(c *infraManager) {
		c.logger = logger
	}
}

func WithInfraStore(store infrastore.Store) InfraManagerOption {
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

func WithLicense(license []byte) InfraManagerOption {
	return func(c *infraManager) {
		c.license = license
	}
}

func WithAirgapBundle(airgapBundle string) InfraManagerOption {
	return func(c *infraManager) {
		c.airgapBundle = airgapBundle
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

func WithKubernetesEnvSettings(kubernetesEnvSettings *helmcli.EnvSettings) InfraManagerOption {
	return func(c *infraManager) {
		c.kubernetesEnvSettings = kubernetesEnvSettings
	}
}

// NewInfraManager creates a new InfraManager with the provided options
func NewInfraManager(opts ...InfraManagerOption) (*infraManager, error) {
	manager := &infraManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.infraStore == nil {
		manager.infraStore = infrastore.NewMemoryStore()
	}

	// If none is provided, use the default env settings from helm
	if manager.kubernetesEnvSettings == nil {
		manager.kubernetesEnvSettings = helmcli.New()
	}

	var restClientGetter genericclioptions.RESTClientGetter
	if manager.kubernetesEnvSettings != nil {
		restClientGetter = manager.kubernetesEnvSettings.RESTClientGetter()
	}

	if manager.kcli == nil {
		kcli, err := clients.NewKubeClient(clients.KubeClientOptions{RESTClientGetter: restClientGetter})
		if err != nil {
			return nil, fmt.Errorf("create kube client: %w", err)
		}
		manager.kcli = kcli
	}

	if manager.mcli == nil {
		mcli, err := clients.NewMetadataClient(clients.KubeClientOptions{RESTClientGetter: restClientGetter})
		if err != nil {
			return nil, fmt.Errorf("create metadata client: %w", err)
		}
		manager.mcli = mcli
	}

	if manager.hcli == nil {
		return nil, fmt.Errorf("helm client is required")
	}

	return manager, nil
}

func (m *infraManager) Get() (types.Infra, error) {
	return m.infraStore.Get()
}
