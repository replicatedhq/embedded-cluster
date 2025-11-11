package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/sirupsen/logrus"
)

// AppPreflightManager provides methods for running app preflights
type AppPreflightManager interface {
	RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) error
	GetAppPreflightStatus(ctx context.Context) (types.Status, error)
	GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error)
	GetAppPreflightTitles(ctx context.Context) ([]string, error)
}

type appPreflightManager struct {
	appPreflightStore preflight.Store
	runner            preflights.PreflightRunnerInterface
	logger            logrus.FieldLogger
}

type AppPreflightManagerOption func(*appPreflightManager)

func WithLogger(logger logrus.FieldLogger) AppPreflightManagerOption {
	return func(m *appPreflightManager) {
		m.logger = logger
	}
}

func WithAppPreflightStore(appPreflightStore preflight.Store) AppPreflightManagerOption {
	return func(m *appPreflightManager) {
		m.appPreflightStore = appPreflightStore
	}
}

func WithPreflightRunner(runner preflights.PreflightRunnerInterface) AppPreflightManagerOption {
	return func(m *appPreflightManager) {
		m.runner = runner
	}
}

// NewAppPreflightManager creates a new AppPreflightManager
func NewAppPreflightManager(opts ...AppPreflightManagerOption) AppPreflightManager {
	manager := &appPreflightManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.appPreflightStore == nil {
		manager.appPreflightStore = preflight.NewMemoryStore()
	}

	if manager.runner == nil {
		manager.runner = preflights.New()
	}

	return manager
}
