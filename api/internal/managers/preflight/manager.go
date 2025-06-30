package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

// HostPreflightManager provides methods for running host preflights
type HostPreflightManager interface {
	PrepareHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error)
	RunHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts RunHostPreflightOptions) error
	GetHostPreflightStatus(ctx context.Context) (types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
}

type hostPreflightManager struct {
	hostPreflightStore preflight.Store
	runner             preflights.PreflightsRunnerInterface
	netUtils           utils.NetUtils
	logger             logrus.FieldLogger
}

type HostPreflightManagerOption func(*hostPreflightManager)

func WithLogger(logger logrus.FieldLogger) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.logger = logger
	}
}

func WithHostPreflightStore(hostPreflightStore preflight.Store) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.hostPreflightStore = hostPreflightStore
	}
}

func WithPreflightRunner(runner preflights.PreflightsRunnerInterface) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.runner = runner
	}
}

func WithNetUtils(netUtils utils.NetUtils) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.netUtils = netUtils
	}
}

// NewHostPreflightManager creates a new HostPreflightManager
func NewHostPreflightManager(opts ...HostPreflightManagerOption) HostPreflightManager {
	manager := &hostPreflightManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.hostPreflightStore == nil {
		manager.hostPreflightStore = preflight.NewMemoryStore()
	}

	if manager.runner == nil {
		manager.runner = preflights.New()
	}

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	return manager
}
