package infra

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ InfraManager = &infraManager{}

// InfraManager provides methods for managing infrastructure setup
type InfraManager interface {
	Get() (types.KubernetesInfra, error)
	Install(ctx context.Context, ki kubernetesinstallation.Installation) error
}

// infraManager is an implementation of the InfraManager interface
type infraManager struct {
	infraStore    infra.Store
	password      string
	releaseData   *release.ReleaseData
	endUserConfig *ecv1beta1.Config
	logger        logrus.FieldLogger
	kcli          client.Client
	mcli          metadata.Interface
	hcli          helm.Client
	mu            sync.RWMutex
}

type InfraManagerOption func(*infraManager)

func WithLogger(logger logrus.FieldLogger) InfraManagerOption {
	return func(c *infraManager) {
		c.logger = logger
	}
}

func WithInfraStore(store infra.Store) InfraManagerOption {
	return func(c *infraManager) {
		c.infraStore = store
	}
}

func WithPassword(password string) InfraManagerOption {
	return func(c *infraManager) {
		c.password = password
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

// NewInfraManager creates a new InfraManager with the provided options
func NewInfraManager(opts ...InfraManagerOption) *infraManager {
	manager := &infraManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.infraStore == nil {
		manager.infraStore = infra.NewMemoryStore()
	}

	return manager
}

func (m *infraManager) Get() (types.KubernetesInfra, error) {
	return m.infraStore.Get()
}
