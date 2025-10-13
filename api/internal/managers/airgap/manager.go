package airgap

import (
	"context"

	airgapstore "github.com/replicatedhq/embedded-cluster/api/internal/store/airgap"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

var _ AirgapManager = &airgapManager{}

// AirgapManager provides methods for managing airgap processing
type AirgapManager interface {
	// Process processes the airgap bundle
	Process(ctx context.Context, registrySettings *types.RegistrySettings) error
	// GetStatus returns the current airgap processing status
	GetStatus() (types.Airgap, error)
}

// airgapManager is an implementation of the AirgapManager interface
type airgapManager struct {
	airgapStore  airgapstore.Store
	airgapBundle string
	clusterID    string
	logger       logrus.FieldLogger
}

type AirgapManagerOption func(*airgapManager)

func WithLogger(logger logrus.FieldLogger) AirgapManagerOption {
	return func(m *airgapManager) {
		m.logger = logger
	}
}

func WithAirgapStore(store airgapstore.Store) AirgapManagerOption {
	return func(m *airgapManager) {
		m.airgapStore = store
	}
}

func WithAirgapBundle(airgapBundle string) AirgapManagerOption {
	return func(m *airgapManager) {
		m.airgapBundle = airgapBundle
	}
}

func WithClusterID(clusterID string) AirgapManagerOption {
	return func(m *airgapManager) {
		m.clusterID = clusterID
	}
}

// NewAirgapManager creates a new AirgapManager with the provided options
func NewAirgapManager(opts ...AirgapManagerOption) (*airgapManager, error) {
	manager := &airgapManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.airgapStore == nil {
		manager.airgapStore = airgapstore.NewMemoryStore()
	}

	return manager, nil
}
